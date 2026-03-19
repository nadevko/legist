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
  ];

  hardeningDisable = [ "fortify" ];

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
  '';
}