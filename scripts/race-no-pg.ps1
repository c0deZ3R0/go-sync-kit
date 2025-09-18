$ErrorActionPreference = 'Stop'

$pkgs = go list ./...
$filtered = @()
foreach ($p in $pkgs) {
  if ($p -notmatch 'storage/postgres($|/)') { $filtered += $p }
}

if ($filtered.Length -eq 0) {
  Write-Host 'No packages to test'
  exit 0
}

& go test $filtered -race -count=1
