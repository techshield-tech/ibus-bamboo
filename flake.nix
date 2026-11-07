{
  description = "A flake for ibus-bamboo";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      version = "v" + builtins.replaceStrings [ "\n" ] [ "" ] (builtins.readFile ./data/bamboo-version);

      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];

      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.stdenv.mkDerivation {
            pname = "ibus-bamboo";
            inherit version;

            src = ./.;

            nativeBuildInputs = [
              pkgs.pkg-config
              pkgs.wrapGAppsHook3
              pkgs.go
            ];

            buildInputs = [
              pkgs.libxtst
            ];

            preConfigure = ''
              export GOCACHE="$TMPDIR/go-cache"
              # Never try to download a newer Go toolchain: the sandbox has no
              # network or $HOME, so fail with a clear version error instead.
              export GOTOOLCHAIN=local
              sed -i "s,/usr,$out," data/bamboo.xml
            '';

            makeFlags = [
              "PREFIX=${placeholder "out"}"
            ];

            meta = {
              isIbusEngine = true;
            };
          };
        }
      );

      devShells = forAllSystems (system:
        let
            pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = [
              pkgs.pkg-config
              pkgs.wrapGAppsHook3
              pkgs.go
            ];

            buildInputs = [
              pkgs.libxtst
            ];
          };
        }
      );
    };
}
