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

      # acpx publishes a prebuilt dist/ to npm, but its repo builds with pnpm and a
      # large dev toolchain, so the tarball is packaged with a runtime-only lockfile.
      # To bump: set acpxVersion and acpxHash, then regenerate the lockfile from the
      # tarball's package/ dir and set npmDepsHash to lib.fakeHash once:
      #   jq 'del(.devDependencies, .scripts)' package.json > p && mv p package.json
      #   npm install --package-lock-only --ignore-scripts
      acpxVersion = "0.19.1";
      acpxHash = "sha256-+Z106BCFEhVjyRf0UJdY+3i/H6MEJORpGTwJg3WSu/A=";

      mkAcpx =
        pkgs:
        pkgs.buildNpmPackage {
          pname = "acpx";
          version = acpxVersion;
          src = pkgs.fetchurl {
            url = "https://registry.npmjs.org/acpx/-/acpx-${acpxVersion}.tgz";
            hash = acpxHash;
          };
          postPatch = ''
            ${pkgs.lib.getExe pkgs.jq} 'del(.devDependencies, .scripts)' package.json > package.json.new
            mv package.json.new package.json
            cp ${./nix/acpx-package-lock.json} package-lock.json
          '';
          npmDepsHash = "sha256-EmjjBR0RKfINF+Biidu3UnM/aAyLNz3KSReS6SC+TYE=";
          dontNpmBuild = true;
          meta = {
            description = "Headless CLI client for the Agent Client Protocol";
            homepage = "https://github.com/openclaw/acpx";
            mainProgram = "acpx";
          };
        };

      pondVersion = "0.19.0";
      pondAssets = {
        x86_64-linux = {
          target = "x86_64-unknown-linux-gnu";
          hash = "sha256-Y3WLDwpUFNd1bhMkIm/yQYWSPPRv8tIIyKZz8U5G2mg=";
        };
        aarch64-linux = {
          target = "aarch64-unknown-linux-gnu";
          hash = "sha256-kTsrtSNSu8nWQYgv6i4FzaniOKD6Q4vI0UY5gdMdfDU=";
        };
        aarch64-darwin = {
          target = "aarch64-apple-darwin";
          hash = "sha256-SLGIkjbZI9ZzQX7oiUjzf0yQL5lc3n+cS96jhaX2PGU=";
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
        acpx = mkAcpx pkgs;
        pond = mkPond pkgs;
        default = pond;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "gunkata";

          packages = [
            (mkAcpx pkgs)
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

          # The executor CLIs are host-provided on purpose: each is installed by its own
          # vendor and cannot be fetched reproducibly.
          shellHook = ''
            gunkata_check() {
              local name="$1" version
              if ! command -v "$name" >/dev/null 2>&1; then
                printf '  %-8s MISSING   (host-provided)\n' "$name"
                return
              fi
              version="$("$name" --version 2>/dev/null | head -n1)"
              printf '  %-8s %s\n' "$name" "''${version:-present}"
            }

            echo "gunkata dev shell"
            echo "flake-pinned:"
            printf '  %-8s %s\n' acpx "$(acpx --version 2>/dev/null | head -n1)"
            printf '  %-8s %s\n' go "$(go version | cut -d' ' -f3)"
            printf '  %-8s %s\n' pond "$(pond --version 2>/dev/null | head -n1)"
            printf '  %-8s %s\n' gitleaks "$(gitleaks version 2>/dev/null | head -n1)"
            printf '  %-8s %s\n' actionlint "$(actionlint --version 2>/dev/null | head -n1)"
            echo "host-provided:"
            gunkata_check claude
            gunkata_check codex
            gunkata_check agy
          '';
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
