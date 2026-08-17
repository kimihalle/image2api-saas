param(
  [int]$Count = 500,
  [int]$StartImage = 31
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$assetDir = Join-Path $root 'frontend/public/inspiration'
$catalogPath = Join-Path $root 'backend/internal/bootstrap/prompt_catalog.json'
$manifestPath = Join-Path $assetDir 'catalog-manifest.json'
$cachePath = Join-Path $PSScriptRoot '.prompt-translation-cache.json'
$headers = @{ 'User-Agent' = 'Vivid-Prompt-Catalog-Builder/1.0' }
$repoRoot = 'https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references'

New-Item -ItemType Directory -Force -Path $assetDir | Out-Null
Add-Type -AssemblyName System.Drawing.Common

$categoryMeta = [ordered]@{
  'profile-avatar' = @{ name = '人像头像'; description = '商业人像、职业形象与个人头像'; icon = 'UserRound' }
  'social-media-post' = @{ name = '社交内容'; description = '社交媒体配图、活动与栏目内容'; icon = 'MessageCircle' }
  'infographic-edu-visual' = @{ name = '知识视觉'; description = '信息图、教学图解与知识可视化'; icon = 'ChartNoAxesColumn' }
  'youtube-thumbnail' = @{ name = '视频封面'; description = '视频频道封面与内容主视觉'; icon = 'PanelsTopLeft' }
  'comic-storyboard' = @{ name = '漫画分镜'; description = '漫画画面、分镜及叙事构图'; icon = 'LayoutPanelTop' }
  'product-marketing' = @{ name = '产品营销'; description = '产品广告、品牌活动与商业视觉'; icon = 'Badge' }
  'ecommerce-main-image' = @{ name = '电商主图'; description = '商品主图、详情页与销售素材'; icon = 'ShoppingBag' }
  'game-asset' = @{ name = '游戏美术'; description = '角色、道具、场景与概念设定'; icon = 'Gamepad2' }
  'poster-flyer' = @{ name = '海报设计'; description = '品牌海报、活动传单与平面视觉'; icon = 'PanelsTopLeft' }
  'app-web-design' = @{ name = '界面设计'; description = '应用界面、网页与数字产品视觉'; icon = 'MonitorSmartphone' }
}

$blocked = '(?i)\b(nude|nudity|lingerie|underwear|bikini|swimsuit|cleavage|breast|fetish|schoolgirl|child|minor|loli|nsfw|sexy|seductive|blood|gore)\b'
$manifest = Invoke-RestMethod -Headers $headers -Uri "$repoRoot/manifest.json" -TimeoutSec 45
$filesBySlug = @{}
foreach ($entry in $manifest.categories) { $filesBySlug[$entry.slug] = $entry.file }

$translationCache = @{}
if (Test-Path $cachePath) {
  try {
    $stored = Get-Content $cachePath -Raw | ConvertFrom-Json -AsHashtable
    if ($stored) { $translationCache = $stored }
  } catch {}
}

function Convert-ToChinese([string]$text) {
  $value = $text.Trim()
  if ([string]::IsNullOrWhiteSpace($value)) { return '' }
  if ($value -match '[\u4e00-\u9fff]') { return $value }
  if ($translationCache.ContainsKey($value)) { return [string]$translationCache[$value] }
  try {
    $query = [uri]::EscapeDataString($value)
    $result = Invoke-RestMethod -Uri "https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=zh-CN&dt=t&q=$query" -TimeoutSec 20
    $translated = (@($result[0]) | ForEach-Object { [string]$_[0] }) -join ''
    if (-not [string]::IsNullOrWhiteSpace($translated)) {
      $translationCache[$value] = $translated.Trim()
      Start-Sleep -Milliseconds 80
      return $translated.Trim()
    }
  } catch {}
  return $value
}

function Get-VariableName([string]$name) {
  $clean = $name.Trim().ToLowerInvariant() -replace '[^a-z0-9_-]+', '_'
  $clean = $clean.Trim('_')
  if (-not $clean) { return 'content' }
  return $clean
}

function Convert-Prompt([string]$content) {
  $variables = [System.Collections.Generic.List[object]]::new()
  $seen = @{}
  $pattern = '\{argument\s+name="([^"]+)"(?:\s+default="([^"]*)")?\}'
  # Some upstream JSON-shaped prompts escape only the quotes inside the
  # argument token. Normalize those tokens before converting them to the
  # template syntax understood by this application.
  $normalizedContent = $content -replace '\\"', '"'
  $prompt = [regex]::Replace($normalizedContent, $pattern, {
    param($match)
    $rawName = $match.Groups[1].Value
    $name = Get-VariableName $rawName
    if (-not $seen.ContainsKey($name)) {
      $default = $match.Groups[2].Value
      $variables.Add([ordered]@{ name = $name; label = $rawName; type = 'text'; default = $default; required = [string]::IsNullOrWhiteSpace($default) })
      $seen[$name] = $true
    }
    return "{{$name}}"
  })
  # A small number of upstream defaults contain nested, unescaped quotes.
  # Keep the variable usable and discard only that malformed default value.
  $prompt = [regex]::Replace($prompt, '\{argument\s+name="([^"]+)"[^}]*\}', {
    param($match)
    $rawName = $match.Groups[1].Value
    $name = Get-VariableName $rawName
    if (-not $seen.ContainsKey($name)) {
      $variables.Add([ordered]@{ name = $name; label = $rawName; type = 'text'; default = ''; required = $true })
      $seen[$name] = $true
    }
    return "{{$name}}"
  })
  return @{ prompt = $prompt.Trim(); variables = @($variables) }
}

function Save-Preview([string]$url, [string]$path) {
  $temp = "$path.download"
  try {
    Invoke-WebRequest -Headers $headers -Uri $url -OutFile $temp -TimeoutSec 45
    if ((Get-Item $temp).Length -lt 2048) { throw 'image is too small' }
    $source = [System.Drawing.Image]::FromFile($temp)
    try {
      $scale = [Math]::Min(1.0, 1200.0 / [Math]::Max($source.Width, $source.Height))
      $width = [Math]::Max(1, [int][Math]::Round($source.Width * $scale))
      $height = [Math]::Max(1, [int][Math]::Round($source.Height * $scale))
      $bitmap = New-Object System.Drawing.Bitmap($width, $height)
      try {
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        try {
          $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
          $graphics.DrawImage($source, 0, 0, $width, $height)
        } finally { $graphics.Dispose() }
        $encoder = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object MimeType -eq 'image/jpeg'
        $parameters = New-Object System.Drawing.Imaging.EncoderParameters(1)
        $parameters.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality, [long]82)
        $bitmap.Save($path, $encoder, $parameters)
      } finally { $bitmap.Dispose() }
    } finally { $source.Dispose() }
    return $true
  } catch {
    return $false
  } finally {
    if (Test-Path $temp) { Remove-Item -LiteralPath $temp -Force }
  }
}

$existingAssetSources = @{}
$existingAssetPathsBySource = @{}
if (Test-Path $manifestPath) {
  try {
    foreach ($item in (Get-Content $manifestPath -Raw | ConvertFrom-Json)) {
      $existingAssetSources[[string]$item.path] = [string]$item.source
      $existingPath = Join-Path $root ('frontend/public' + ([string]$item.path).Replace('/', '\'))
      if (Test-Path $existingPath) { $existingAssetPathsBySource[[string]$item.source] = $existingPath }
    }
  } catch {}
}

$perCategory = [Math]::Floor($Count / $categoryMeta.Count)
$remainder = $Count % $categoryMeta.Count
$catalog = [System.Collections.Generic.List[object]]::new()
$assetManifest = [System.Collections.Generic.List[object]]::new()
$usedTitles = @{}
$usedMedia = @{}
$usedHashes = @{}
$baseManifestPath = Join-Path $assetDir 'manifest.json'
if (Test-Path $baseManifestPath) {
  try {
    foreach ($item in (Get-Content $baseManifestPath -Raw | ConvertFrom-Json)) {
      if ($item.source) { $usedMedia[[string]$item.source] = $true }
    }
  } catch {}
}
foreach ($baseImage in (Get-ChildItem $assetDir -Filter '*.jpg' | Where-Object { $_.BaseName.Length -eq 2 })) {
  $usedHashes[(Get-FileHash $baseImage.FullName -Algorithm SHA256).Hash] = $true
}
$categoryIndex = 0

foreach ($slug in $categoryMeta.Keys) {
  $target = $perCategory + $(if ($categoryIndex -lt $remainder) { 1 } else { 0 })
  $categoryIndex++
  $records = Invoke-RestMethod -Headers $headers -Uri "$repoRoot/$($filesBySlug[$slug])" -TimeoutSec 120
  $accepted = 0
  foreach ($record in $records) {
    if ($accepted -ge $target) { break }
    $title = ([string]$record.title).Trim()
    $description = ([string]$record.description).Trim()
    $content = ([string]$record.content).Trim()
    $media = @($record.sourceMedia | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) | Select-Object -First 1
    if (-not $title -or -not $content -or -not $media) { continue }
    if ($title.Length -gt 140 -or $content.Length -gt 12000) { continue }
    if (("$title $description $content") -match $blocked) { continue }
    $titleKey = $title.ToLowerInvariant()
    if ($usedTitles.ContainsKey($titleKey) -or $usedMedia.ContainsKey($media)) { continue }

    $sourceID = [string]$record.id
    $fileName = "ym-$sourceID.jpg"
    $imagePath = Join-Path $assetDir $fileName
    $cover = "/inspiration/$fileName"
    $canReuse = (Test-Path $imagePath) -and $existingAssetSources.ContainsKey($cover) -and $existingAssetSources[$cover] -eq $media
    if (-not $canReuse -and $existingAssetPathsBySource.ContainsKey($media)) {
      Copy-Item -LiteralPath $existingAssetPathsBySource[$media] -Destination $imagePath -Force
      $canReuse = $true
    }
    if (-not $canReuse -and (Test-Path $imagePath)) { Remove-Item -LiteralPath $imagePath -Force }
    if (-not $canReuse -and -not (Save-Preview $media $imagePath)) { continue }
    $imageHash = (Get-FileHash $imagePath -Algorithm SHA256).Hash
    if ($usedHashes.ContainsKey($imageHash)) {
      Remove-Item -LiteralPath $imagePath -Force
      continue
    }
    $converted = Convert-Prompt $content
    $localizedTitle = Convert-ToChinese $title
    $localizedDescription = Convert-ToChinese $description
    if (-not $localizedDescription) { $localizedDescription = $categoryMeta[$slug].description }
    $catalog.Add([ordered]@{
      id = "pt-youmind-$sourceID"
      source_id = $sourceID
      category_slug = $slug
      title = $localizedTitle
      description = $localizedDescription
      source_title = $title
      prompt = $converted.prompt
      variables = $converted.variables
      cover = $cover
      need_reference_images = [bool]$record.needReferenceImages
    })
    $assetManifest.Add([ordered]@{ path = $cover; source = $media; source_id = $sourceID; source_title = $title; title = $localizedTitle; category = $slug })
    $usedTitles[$titleKey] = $true
    $usedMedia[$media] = $true
    $usedHashes[$imageHash] = $true
    $accepted++
    Write-Progress -Activity '构建灵感模板目录' -Status "$($catalog.Count) / $Count" -PercentComplete (($catalog.Count / $Count) * 100)
  }
  if ($accepted -lt $target) { throw "Category $slug only produced $accepted of $target templates" }
}

if ($catalog.Count -ne $Count) { throw "Expected $Count templates, generated $($catalog.Count)" }
$activeCovers = @{}
foreach ($item in $assetManifest) { $activeCovers[[string]$item.path] = $true }
foreach ($candidate in (Get-ChildItem $assetDir -Filter '*.jpg' | Where-Object { $_.BaseName.Length -eq 3 -or $_.BaseName.StartsWith('ym-') })) {
  $candidateCover = "/inspiration/$($candidate.Name)"
  if (-not $activeCovers.ContainsKey($candidateCover)) { Remove-Item -LiteralPath $candidate.FullName -Force }
}
$catalog | ConvertTo-Json -Depth 10 | Set-Content -Encoding UTF8 $catalogPath
$assetManifest | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $manifestPath
$translationCache | ConvertTo-Json -Depth 3 | Set-Content -Encoding UTF8 $cachePath
Write-Progress -Activity '构建灵感模板目录' -Completed
Write-Output "Generated $($catalog.Count) templates and $($assetManifest.Count) local preview images."
