param(
    [string]$LanIP = "",
    [string[]]$ProcessPattern = @("*Forest*", "*SOTF*", "*Sons*"),
    [int]$DurationSeconds = 240,
    [int]$IntervalMs = 500,
    [string]$OutDir = "",
    [switch]$NoPktmon
)

$ErrorActionPreference = "Stop"

function Test-IsAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-PreferredLanIPv4 {
    $routes = @(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix "0.0.0.0/0" -ErrorAction SilentlyContinue |
        Sort-Object RouteMetric, InterfaceMetric)
    foreach ($route in $routes) {
        $address = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $route.InterfaceIndex -ErrorAction SilentlyContinue |
            Where-Object {
                $_.IPAddress -notlike "127.*" -and
                $_.IPAddress -notlike "169.254.*" -and
                $_.IPAddress -ne "0.0.0.0"
            } |
            Select-Object -First 1
        if ($null -ne $address) {
            return $address.IPAddress
        }
    }
    return ""
}

function Get-MatchingProcesses {
    $patterns = $ProcessPattern
    Get-Process | Where-Object {
        $processName = $_.ProcessName
        $matched = $false
        foreach ($pattern in $patterns) {
            if ($processName -like $pattern) {
                $matched = $true
                break
            }
        }
        $matched
    } | Sort-Object ProcessName, Id
}

function New-SampleObject {
    param(
        [datetime]$Timestamp,
        [object]$Process,
        [object]$Endpoint,
        [string]$Protocol
    )

    $item = [ordered]@{
        timestamp = $Timestamp.ToString("o")
        protocol = $Protocol
        process_name = $Process.ProcessName
        pid = $Process.Id
        local_address = $Endpoint.LocalAddress
        local_port = $Endpoint.LocalPort
        remote_address = ""
        remote_port = ""
        state = ""
    }

    if ($Protocol -eq "tcp") {
        $item.remote_address = $Endpoint.RemoteAddress
        $item.remote_port = $Endpoint.RemotePort
        $item.state = $Endpoint.State
    }

    [pscustomobject]$item
}

if ($DurationSeconds -lt 5) {
    throw "DurationSeconds must be at least 5."
}

if ($IntervalMs -lt 100) {
    throw "IntervalMs must be at least 100."
}

if ([string]::IsNullOrWhiteSpace($LanIP)) {
    $LanIP = Get-PreferredLanIPv4
}
if ([string]::IsNullOrWhiteSpace($LanIP)) {
    throw "Could not determine a LAN IPv4 address. Run again with -LanIP <address>."
}

$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $OutDir = Join-Path $repoRoot "logs\sotf-capture-$stamp"
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

$isAdmin = Test-IsAdmin
$pktmon = Get-Command pktmon -ErrorAction SilentlyContinue
$usePktmon = (-not $NoPktmon) -and $isAdmin -and ($null -ne $pktmon)
$pktmonStarted = $false
$etlPath = Join-Path $OutDir "pktmon.etl"
$pcapPath = Join-Path $OutDir "pktmon.pcapng"
$pktmonTxtPath = Join-Path $OutDir "pktmon.txt"
$summaryPath = Join-Path $OutDir "summary.md"
$udpCsvPath = Join-Path $OutDir "udp-endpoints.csv"
$tcpCsvPath = Join-Path $OutDir "tcp-connections.csv"
$processCsvPath = Join-Path $OutDir "processes.csv"
$metadataPath = Join-Path $OutDir "metadata.json"

$metadata = [ordered]@{
    started_at = (Get-Date).ToString("o")
    lan_ip = $LanIP
    process_patterns = $ProcessPattern
    duration_seconds = $DurationSeconds
    interval_ms = $IntervalMs
    admin = $isAdmin
    pktmon_available = ($null -ne $pktmon)
    pktmon_enabled = $usePktmon
    pktmon_raw_capture_may_contain_payload = $usePktmon
}
$metadata | ConvertTo-Json -Depth 5 | Set-Content -Path $metadataPath -Encoding utf8

$udpSamples = New-Object System.Collections.Generic.List[object]
$tcpSamples = New-Object System.Collections.Generic.List[object]
$processSamples = New-Object System.Collections.Generic.List[object]

Write-Host "SOTF port capture"
Write-Host "Output: $OutDir"
Write-Host "LAN IP filter: $LanIP"
Write-Host "Process patterns: $($ProcessPattern -join ', ')"
Write-Host "Duration: $DurationSeconds seconds"

if (-not $isAdmin) {
    Write-Host "Not running as Administrator. Endpoint CSVs will be collected, but pktmon PCAP capture is disabled."
}

if ($NoPktmon) {
    Write-Host "Pktmon disabled by -NoPktmon."
}

try {
    if ($usePktmon) {
        Write-Host "Starting bounded pktmon capture. Existing global pktmon filters will not be changed."
        & pktmon start --capture --comp all --pkt-size 256 --file-name $etlPath --file-size 64 --log-mode circular | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "pktmon could not start. It may already be in use by another capture."
        }
        $pktmonStarted = $true
    }

    Write-Host ""
    Write-Host "Now do the test:"
    Write-Host "1. Refresh the SOTF lobby/server list."
    Write-Host "2. Join the dedicated server through $LanIP."
    Write-Host "3. Walk around for a short time."
    Write-Host "4. Exit back out."
    Write-Host ""
    Write-Host "Press Ctrl+C to stop early."

    $started = Get-Date
    while (((Get-Date) - $started).TotalSeconds -lt $DurationSeconds) {
        $now = Get-Date
        $processes = @(Get-MatchingProcesses)
        $pids = @($processes | Select-Object -ExpandProperty Id)

        foreach ($process in $processes) {
            $processPath = ""
            $processStartTime = ""
            try { $processPath = $process.Path } catch { $processPath = "" }
            try { $processStartTime = $process.StartTime.ToString("o") } catch { $processStartTime = "" }

            $processSamples.Add([pscustomobject][ordered]@{
                timestamp = $now.ToString("o")
                process_name = $process.ProcessName
                pid = $process.Id
                path = $processPath
                start_time = $processStartTime
            })
        }

        if ($pids.Count -gt 0) {
            foreach ($processId in $pids) {
                $process = $processes | Where-Object { $_.Id -eq $processId } | Select-Object -First 1

                $udpEndpoints = @(Get-NetUDPEndpoint -OwningProcess $processId -ErrorAction SilentlyContinue)
                foreach ($endpoint in $udpEndpoints) {
                    $udpSamples.Add((New-SampleObject -Timestamp $now -Process $process -Endpoint $endpoint -Protocol "udp"))
                }

                $tcpConnections = @(Get-NetTCPConnection -OwningProcess $processId -ErrorAction SilentlyContinue)
                foreach ($connection in $tcpConnections) {
                    $tcpSamples.Add((New-SampleObject -Timestamp $now -Process $process -Endpoint $connection -Protocol "tcp"))
                }
            }
        }

        $elapsed = [int]((Get-Date) - $started).TotalSeconds
        if (($elapsed % 5) -eq 0) {
            $portPreview = @($udpSamples | Select-Object -Last 20 | Select-Object -ExpandProperty local_port -Unique) -join ","
            if ([string]::IsNullOrWhiteSpace($portPreview)) {
                $portPreview = "none yet"
            }
            Write-Host ("[{0,3}s/{1}s] matched_processes={2} recent_udp_ports={3}" -f $elapsed, $DurationSeconds, $processes.Count, $portPreview)
        }

        Start-Sleep -Milliseconds $IntervalMs
    }
}
finally {
    if ($pktmonStarted) {
        Write-Host "Stopping pktmon capture ..."
        try { & pktmon stop | Out-Null } catch { Write-Warning $_.Exception.Message }
        if (Test-Path $etlPath) {
            try { & pktmon etl2pcap $etlPath --out $pcapPath | Out-Null } catch { Write-Warning $_.Exception.Message }
            try { & pktmon etl2txt $etlPath --out $pktmonTxtPath --brief | Out-Null } catch { Write-Warning $_.Exception.Message }
        }
    }

    if ($processSamples.Count -gt 0) {
        $processSamples | Export-Csv -Path $processCsvPath -NoTypeInformation -Encoding utf8
    }
    if ($udpSamples.Count -gt 0) {
        $udpSamples | Export-Csv -Path $udpCsvPath -NoTypeInformation -Encoding utf8
    }
    if ($tcpSamples.Count -gt 0) {
        $tcpSamples | Export-Csv -Path $tcpCsvPath -NoTypeInformation -Encoding utf8
    }

    $udpSummary = @()
    if ($udpSamples.Count -gt 0) {
        $udpSummary = $udpSamples |
            Group-Object process_name, pid, local_address, local_port |
            ForEach-Object {
                $first = $_.Group | Select-Object -First 1
                $last = $_.Group | Select-Object -Last 1
                [pscustomobject][ordered]@{
                    process_name = $first.process_name
                    pid = $first.pid
                    protocol = "udp"
                    local_address = $first.local_address
                    local_port = $first.local_port
                    samples = $_.Count
                    first_seen = $first.timestamp
                    last_seen = $last.timestamp
                }
            } | Sort-Object {[int]$_.local_port}, process_name
    }

    $tcpSummary = @()
    if ($tcpSamples.Count -gt 0) {
        $tcpSummary = $tcpSamples |
            Group-Object process_name, pid, local_address, local_port, remote_address, remote_port, state |
            ForEach-Object {
                $first = $_.Group | Select-Object -First 1
                $last = $_.Group | Select-Object -Last 1
                [pscustomobject][ordered]@{
                    process_name = $first.process_name
                    pid = $first.pid
                    protocol = "tcp"
                    local_address = $first.local_address
                    local_port = $first.local_port
                    remote_address = $first.remote_address
                    remote_port = $first.remote_port
                    state = $first.state
                    samples = $_.Count
                    first_seen = $first.timestamp
                    last_seen = $last.timestamp
                }
            } | Sort-Object process_name, {[int]$_.local_port}, remote_address, remote_port
    }

    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("# SOTF Port Capture Summary")
    $lines.Add("")
    $lines.Add("- Started: $($metadata.started_at)")
    $lines.Add("- LAN IP filter: $LanIP")
    $lines.Add("- Duration: $DurationSeconds seconds")
    $lines.Add("- Admin: $isAdmin")
    $lines.Add("- Pktmon enabled: $usePktmon")
    $lines.Add("")
    $lines.Add("## UDP Local Ports")
    $lines.Add("")
    if ($udpSummary.Count -eq 0) {
        $lines.Add("No matching UDP endpoints were observed.")
    } else {
        $lines.Add("| Process | PID | Local Address | Local Port | Samples | First Seen | Last Seen |")
        $lines.Add("|---|---:|---|---:|---:|---|---|")
        foreach ($row in $udpSummary) {
            $lines.Add("| $($row.process_name) | $($row.pid) | $($row.local_address) | $($row.local_port) | $($row.samples) | $($row.first_seen) | $($row.last_seen) |")
        }
    }
    $lines.Add("")
    $lines.Add("## TCP Connections")
    $lines.Add("")
    if ($tcpSummary.Count -eq 0) {
        $lines.Add("No matching TCP connections were observed.")
    } else {
        $lines.Add("| Process | PID | Local | Remote | State | Samples |")
        $lines.Add("|---|---:|---|---|---|---:|")
        foreach ($row in $tcpSummary) {
            $lines.Add("| $($row.process_name) | $($row.pid) | $($row.local_address):$($row.local_port) | $($row.remote_address):$($row.remote_port) | $($row.state) | $($row.samples) |")
        }
    }
    $lines.Add("")
    $lines.Add("## Files")
    $lines.Add("")
    $lines.Add("- metadata.json")
    $lines.Add("- processes.csv")
    $lines.Add("- udp-endpoints.csv")
    $lines.Add("- tcp-connections.csv")
    if ($pktmonStarted) {
        $lines.Add("- pktmon.etl")
        $lines.Add("- pktmon.pcapng")
        $lines.Add("- pktmon.txt")
    }
    $lines | Set-Content -Path $summaryPath -Encoding utf8

    Write-Host ""
    Write-Host "Capture complete."
    Write-Host "Summary: $summaryPath"
    Write-Host "Output:  $OutDir"
}
