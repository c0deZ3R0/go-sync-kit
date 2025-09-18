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

# Invoke go test on all filtered packages
& go test $filtered -count=1
