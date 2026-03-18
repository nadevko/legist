{ pkgs, ... }:
{
  services.qdrant = {
    enable = true;
    settings = {
      storage = {
        storage_path = "/var/lib/qdrant/storage";
        snapshots_path = "/var/lib/qdrant/snapshots";
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
}