$ErrorActionPreference = 'Stop'

Get-ChildItem -Recurse -Include *.go -File |
  ForEach-Object {
    & gofmt -w $_.FullName
  }
