{
  description = "User-controlled soft autopilot for AI coding";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = if (self ? shortRev) then self.shortRev else "dev";
        go_1_26_4 = pkgs.go_1_26.overrideAttrs (finalAttrs: previousAttrs: {
          version = "1.26.4";
          src = pkgs.fetchurl {
            url = "https://go.dev/dl/go${finalAttrs.version}.src.tar.gz";
            hash = "sha256-T2aKMvv8ETLmqIH7lowvHa2mMUkqM5IRc1+7JVpCYC0=";
          };
        });
        buildGoModule = pkgs.buildGoModule.override { go = go_1_26_4; };
      in
      {
        packages = {
          slipway = buildGoModule {
            pname = "slipway";
            inherit version;
            src = ./.;

            subPackages = [ "." ];
            vendorHash = "sha256-F1mMsdd/txTLkyizOiZSl5cfeC7GGPeSRTN6WhVPiBo=";
            doCheck = false;

            ldflags = [
              "-s"
              "-w"
              "-X github.com/signalridge/slipway/cmd.version=${version}"
              "-X github.com/signalridge/slipway/cmd.commit=${version}"
              "-X github.com/signalridge/slipway/cmd.date=1970-01-01T00:00:00Z"
            ];

            meta = with pkgs.lib; {
              description = "User-controlled soft autopilot for AI coding";
              homepage = "https://github.com/signalridge/slipway";
              license = licenses.bsd3;
              maintainers = [ ];
              mainProgram = "slipway";
            };
          };

          default = self.packages.${system}.slipway;
        };

        apps = {
          slipway = flake-utils.lib.mkApp {
            drv = self.packages.${system}.slipway;
          };
          default = self.apps.${system}.slipway;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_26_4
            gopls
            golangci-lint
            goreleaser
            govulncheck
          ];

          shellHook = ''
            echo "slipway development shell"
            echo "Go version: $(go version)"
          '';
        };
      }
    )
    // {
      overlays.default = final: prev: {
        slipway = self.packages.${prev.system}.slipway;
      };
    };
}
