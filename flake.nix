{
  description = "gunkata - DAG orchestrator for isolated agent executors";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      pondVersion = "0.18.0";
      pondAssets = {
        x86_64-linux = {
          target = "x86_64-unknown-linux-gnu";
          hash = "sha256-r6JHUwnptf84ikyCIi2ssfYEtliSl2oaeznvSMxM7aQ=";
        };
        aarch64-linux = {
          target = "aarch64-unknown-linux-gnu";
          hash = "sha256-koyagHn8nvy+eAkVxHpQUxtL8j4qg1Vk5npKrR6cJWc=";
        };
        aarch64-darwin = {
          target = "aarch64-apple-darwin";
          hash = "sha256-S9XDqeEOQLd8JUQY3KlLa5m17YAfohILmodp1kGoXRg=";
        };
      };

      mkPond =
        pkgs:
        let
          asset = pondAssets.${pkgs.stdenv.hostPlatform.system};
        in
        pkgs.stdenv.mkDerivation {
          pname = "pond";
          version = pondVersion;

          src = pkgs.fetchurl {
            url = "https://github.com/tenequm/pond/releases/download/v${pondVersion}/pond-${asset.target}.tar.xz";
            inherit (asset) hash;
          };

          sourceRoot = ".";

          nativeBuildInputs = [
            pkgs.installShellFiles
          ] ++ pkgs.lib.optional pkgs.stdenv.hostPlatform.isLinux pkgs.autoPatchelfHook;

          installPhase = ''
            runHook preInstall
            install -Dm755 pond "$out/bin/pond"
            installShellCompletion \
              --bash completions/pond.bash \
              --fish completions/pond.fish \
              --zsh completions/_pond
            runHook postInstall
          '';

          meta = {
            description = "Session transcript store and search engine for coding agents";
            homepage = "https://github.com/tenequm/pond";
            mainProgram = "pond";
            platforms = builtins.attrNames pondAssets;
            sourceProvenance = [ pkgs.lib.sourceTypes.binaryNativeCode ];
          };
        };
    in
    {
      packages = forAllSystems (pkgs: rec {
        pond = mkPond pkgs;
        default = pond;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "gunkata";

          packages = [
            pkgs.actionlint
            pkgs.gitleaks
            pkgs.go_1_27
            pkgs.golangci-lint
            pkgs.gotestsum
            pkgs.govulncheck
            pkgs.just
            pkgs.lefthook
            pkgs.python3
            (mkPond pkgs)
          ];

          # acpx and the executor CLIs are host-provided on purpose: acpx ships as an npm
          # package built with pnpm, and an executor's ACP server is installed by the
          # executor's own vendor and cannot be fetched reproducibly. See
          # docs/knowledge/findings/runtime-deps-not-yet-flake-pinned.md
          shellHook = ''
            gunkata_check() {
              local name="$1" floor="$2" version
              if ! command -v "$name" >/dev/null 2>&1; then
                printf '  %-8s MISSING   (host-provided%s)\n' "$name" \
                  "''${floor:+, need >= $floor}"
                return
              fi
              version="$("$name" --version 2>/dev/null | head -n1)"
              printf '  %-8s %s\n' "$name" "''${version:-present}"
            }

            echo "gunkata dev shell"
            echo "flake-pinned:"
            printf '  %-8s %s\n' go "$(go version | cut -d' ' -f3)"
            printf '  %-8s %s\n' pond "$(pond --version 2>/dev/null | head -n1)"
            printf '  %-8s %s\n' gitleaks "$(gitleaks version 2>/dev/null | head -n1)"
            printf '  %-8s %s\n' actionlint "$(actionlint --version 2>/dev/null | head -n1)"
            echo "host-provided:"
            gunkata_check acpx 0.15.0
            gunkata_check claude ""
            gunkata_check codex ""
            gunkata_check agy ""
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
