{
  mkShell,
  go,
  go-swag,
  delve,
  golangci-lint,
  gomod2nix,
  gopls,
  gotools,
  poppler-utils,
  railway,
}: mkShell {
  packages = [
    delve
    go
    go-swag
    golangci-lint
    gomod2nix
    gopls
    gotools
    poppler-utils
    railway
  ];

  hardeningDisable = [ "fortify" ];

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
  '';
}