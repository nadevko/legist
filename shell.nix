{
  mkShell,
  go,
  gopls,
  gotools,
  delve,
  golangci-lint,
}:
mkShell {
  packages = [
    go
    gopls
    gotools
    delve
    golangci-lint
  ];

  GOPATH = "${toString ./.}/.go";
  GOROOT = "${go}/share/go";

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
    echo "Go environment is ready. Go version: $(go version)"
  '';
}
