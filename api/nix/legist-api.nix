{
  buildGoApplication,
  makeWrapper,
  poppler-utils,
}:
buildGoApplication {
  pname = "legist-api";
  version = "0.1";
  src = ../.;
  modules = ./gomod2nix.toml;

  nativeBuildInputs = [ makeWrapper ];
  runtimeDeps = [ poppler-utils ];

  postInstall = ''
    wrapProgram "$out/bin/legist-api" \
      --prefix PATH : ${poppler-utils}/bin
  '';
}
