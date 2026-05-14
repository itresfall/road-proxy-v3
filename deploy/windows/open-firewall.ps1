param(
  [int[]]$TcpPorts = @(8080, 8081),
  [string]$NamePrefix = "ROAD Proxy v3",
  [switch]$WhatIfOnly
)

$ErrorActionPreference = "Stop"

foreach ($port in $TcpPorts) {
  if ($port -lt 1 -or $port -gt 65535) {
    throw "invalid TCP port: $port"
  }

  $ruleName = "ROAD-Proxy-v3-TCP-$port"
  $displayName = "$NamePrefix TCP $port"
  $existing = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue

  if ($WhatIfOnly) {
    if ($existing) {
      Write-Host "Would update firewall rule: $displayName"
    } else {
      Write-Host "Would create firewall rule: $displayName"
    }
    continue
  }

  if ($existing) {
    Set-NetFirewallRule -Name $ruleName -Enabled True -Profile Any -Action Allow
    Write-Host "Updated firewall rule: $displayName"
  } else {
    New-NetFirewallRule -Name $ruleName -DisplayName $displayName -Direction Inbound -Protocol TCP -LocalPort $port -Action Allow -Profile Any | Out-Null
    Write-Host "Created firewall rule: $displayName"
  }
}
