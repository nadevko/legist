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
    mkdir -p "$out/share/legist-api/internal/config"
    cp "${../internal/config/weights-seed.json}" "$out/share/legist-api/internal/config/weights-seed.json"
    cp "${../internal/config/omits-seed.json}" "$out/share/legist-api/internal/config/omits-seed.json"

    wrapProgram "$out/bin/legist-api" \
      --prefix PATH : ${poppler-utils}/bin
  '';
}
