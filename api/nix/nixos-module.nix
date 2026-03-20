{ pkgs, config, lib, ... }:
let
  cfg = config.services.legist-api;
in
{
  options.services.legist-api = {
    enable = lib.mkEnableOption "legist-api service";
    package = lib.mkPackageOption pkgs "legist-api" { };
    addr = lib.mkOption {
      type = lib.types.str;
      default = ":8080";
      description = "Listen address for the API server";
    };
    dataPath = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/legist/data";
      description = "Path to data directory";
    };
    jwtSecret = lib.mkOption {
      type = lib.types.str;
      default = "change-me-in-prod";
      description = "JWT secret for authentication";
    };
    publicHost = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Public host for API";
    };
    basePath = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Base path for API routes";
    };
    env = lib.mkOption {
      type = lib.types.str;
      default = "prod";
      description = "Environment (dev or prod)";
    };
  };

  config = lib.mkIf cfg.enable {
    services.qdrant = {
      enable = true;
      settings = {
        storage = {
          storage_path = "/var/lib/legist/qdrant-storage";
          snapshots_path = "/var/lib/legist/qdrant-snapshots";
        };
        hnsw_index = {
          on_disk = true;
        };
        service = {
          host = "127.0.0.1";
          http_port = 6333;
          grpc_port = 6334;
        };
        telemetry_disabled = true;
      };
    };

    services.ollama = {
      enable = true;
      package = pkgs.ollama-rocm;
      host = "127.0.0.1";
      port = 11434;
      rocmOverrideGfx = "10.3.5";
      loadModels = [
        "nomic-embed-text"
        "qwen2.5:3b"
        "qwen2.5:7b"
      ];
    };

    systemd.services.legist-api = {
      description = "Legist API Server";
      after = [ "network-online.target" "qdrant.service" "ollama.service" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        Type = "simple";
        User = "legist";
        Group = "legist";
        StateDirectory = "legist-api";
        StateDirectoryMode = "0755";
        WorkingDirectory = "/var/lib/legist";
        Restart = "on-failure";
        RestartSec = "5s";

        Environment = [
          "ADDR=${cfg.addr}"
          "DATA_PATH=${cfg.dataPath}"
          "JWT_SECRET=${cfg.jwtSecret}"
          "PUBLIC_HOST=${cfg.publicHost}"
          "BASE_PATH=${cfg.basePath}"
          "ENV=${cfg.env}"
          "WEIGHT_REGEX_FILE=${cfg.package}/share/legist-api/internal/config/weights-seed.json"
          "DIFF_MATCH_REGEX_FILE=${cfg.package}/share/legist-api/internal/config/omits-seed.json"
          "QDRANT_HOST=127.0.0.1"
          "QDRANT_GRPC_PORT=6334"
          "OLLAMA_BASE_URL=http://127.0.0.1:11434"
          "EMBED_MODEL=nomic-embed-text"
          "METADATA_MODEL=qwen2.5:3b"
          "ANALYSIS_MODEL=qwen2.5:7b"
        ];

        ExecStart = "${cfg.package}/bin/legist-api";

        # Безопасность
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [ "/var/lib/legist" ];
      };
    };

    users.users.legist = {
      isSystemUser = true;
      group = "legist";
      description = "Legist API Service User";
    };

    users.groups.legist = { };
  };
}