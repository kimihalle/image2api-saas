$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$target = Join-Path $root 'frontend/public/inspiration'
New-Item -ItemType Directory -Force -Path $target | Out-Null
Get-ChildItem -Path $target -Filter '*.jpg' -ErrorAction SilentlyContinue | Remove-Item -Force
$headers = @{ 'User-Agent' = 'Vivid-Prompt-Asset-Importer/1.0' }
$manifestUrl = 'https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references/manifest.json'
$manifest = Invoke-RestMethod -Headers $headers -Uri $manifestUrl -TimeoutSec 30
$items = [System.Collections.Generic.List[object]]::new()
foreach ($category in $manifest.categories) {
  $url = "https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references/$($category.file)"
  try { $records = Invoke-RestMethod -Headers $headers -Uri $url -TimeoutSec 45 } catch { continue }
  $categoryCount = 0
  foreach ($record in $records) {
    foreach ($media in @($record.sourceMedia)) {
      if ([string]::IsNullOrWhiteSpace($media)) { continue }
      if ($items.url -contains $media) { continue }
      $items.Add([pscustomobject]@{ url = $media; title = $record.title; category = $category.slug })
      $categoryCount++
      if ($categoryCount -ge 3 -or $items.Count -ge 36) { break }
    }
    if ($categoryCount -ge 3 -or $items.Count -ge 36) { break }
  }
  if ($items.Count -ge 28) { break }
}
$out = [System.Collections.Generic.List[object]]::new()
$index = 1
foreach ($item in $items) {
  $name = ('{0:D2}.jpg' -f $index)
  $path = Join-Path $target $name
  try {
    Invoke-WebRequest -Headers $headers -Uri $item.url -OutFile $path -TimeoutSec 45
    if ((Get-Item $path).Length -lt 1024) { Remove-Item $path -Force; continue }
    $out.Add([pscustomobject]@{ path = "/inspiration/$name"; source = $item.url; title = $item.title; category = $item.category })
    $index++
  } catch { if (Test-Path $path) { Remove-Item $path -Force } }
}
$out | ConvertTo-Json -Depth 4 | Set-Content -Encoding UTF8 (Join-Path $target 'manifest.json')
Write-Output "Downloaded $($out.Count) prompt preview assets to $target"
