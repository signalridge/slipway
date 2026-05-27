{
  description = "Governance CLI for AI-assisted software delivery";

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
      in
      {
        packages = {
          slipway = pkgs.buildGoModule {
            pname = "slipway";
            inherit version;
            src = ./.;

            subPackages = [ "." ];
            vendorHash = "sha256-+543pXiTi9cgNmcQr3oyPr7on7BTBjgE36qR96ZR/VY=";
            doCheck = false;

            ldflags = [
              "-s"
              "-w"
              "-X github.com/signalridge/slipway/cmd.version=${version}"
              "-X github.com/signalridge/slipway/cmd.commit=${version}"
              "-X github.com/signalridge/slipway/cmd.date=1970-01-01T00:00:00Z"
            ];

            meta = with pkgs.lib; {
              description = "Governance CLI for AI-assisted software delivery";
              homepage = "https://github.com/signalridge/slipway";
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
            go
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
