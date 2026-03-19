{ legist-api, dockerTools }:
dockerTools.buildImage {
  name = "legist-api";
  tag = "0.1";
  contents = [ legist-api ];
  config = {
    Cmd = [ "${legist-api}/bin/legist-api" ];
    Env = [ "PORT=8080" ];
    Expose = [ 8080 ];
  };
}
