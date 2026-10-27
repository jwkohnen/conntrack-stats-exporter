{
  description = "conntrack-stats-exporter, packaged as a minimal OCI image";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forEachSystem = f: nixpkgs.lib.genAttrs systems (system: f { inherit system; });
    in
    {
      packages = forEachSystem (
        { system }:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          inherit (pkgs) lib;

          # nixos-26.05 defaults to Go 1.26, but ships Go 1.27.3 as
          # `go_1_27`; use its matching module builder to pin the toolchain.
          conntrack-stats-exporter = pkgs.buildGo127Module {
            pname = "conntrack-stats-exporter";
            version = "unstable-${self.shortRev or "dirty"}";

            src = self;

            # `go test ./...` executes a mocked conntrack script via a
            # `#!/bin/sh` shebang, which does not exist inside the Nix build
            # sandbox. Tests are run by CI instead.
            doCheck = false;

            vendorHash = "sha256-6aZIN8cVbG3NYBFrd1NOIEDWd5cgWPxGl2HyyDGQrRM=";

            env.CGO_ENABLED = "0";

            # Strip symbols and the build ID for a smaller, reproducible binary.
            ldflags = [
              "-s"
              "-w"
            ];

            meta = {
              description = "Prometheus exporter for Linux conntrack statistics";
              homepage = "https://github.com/jwkohnen/conntrack-stats-exporter";
              license = lib.licenses.gpl3Plus;
              mainProgram = "conntrack-stats-exporter";
            };

            # Nix's Go is patched to embed paths to tzdata, iana-etc and
            # mailcap. This exporter uses none of them, so drop the references
            # and keep those closures out of the image.
            postInstall = ''
              ${pkgs.removeReferencesTo}/bin/remove-references-to \
                -t ${pkgs.iana-etc} \
                -t ${pkgs.mailcap} \
                -t ${pkgs.tzdata} \
                $out/bin/conntrack-stats-exporter
            '';
          };

          # Build conntrack statically so the image does not need a dynamic
          # loader or shared libraries. systemd support is irrelevant here and
          # would only drag in extra dependencies.
          conntrack =
            let
              conntrack-tools = pkgs.pkgsStatic.conntrack-tools.override { systemdSupport = false; };
            in
            pkgs.runCommand "conntrack-only" { } ''
              install -Dm0755 ${conntrack-tools}/bin/conntrack $out/bin/conntrack
              ${pkgs.binutils-unwrapped}/bin/strip --strip-all $out/bin/conntrack
            '';

          image = pkgs.dockerTools.buildLayeredImage {
            name = "conntrack-stats-exporter";
            tag = "latest";
            created = "1970-01-01T00:00:01Z";

            # Both packages are static, so the image contains little more than
            # the two binaries themselves.
            contents = [
              conntrack-stats-exporter
              conntrack
            ];

            config = {
              Entrypoint = [ "/bin/conntrack-stats-exporter" ];
              Env = [ "PATH=/bin" ];
              ExposedPorts."9371/tcp" = { };
              WorkingDir = "/";
              Labels = {
                "org.opencontainers.image.source" = "https://github.com/jwkohnen/conntrack-stats-exporter";
                "org.opencontainers.image.title" = "conntrack-stats-exporter";
                "org.opencontainers.image.licenses" = "GPL-3.0-or-later";
              };
            };
          };
        in
        {
          inherit conntrack-stats-exporter conntrack image;
          default = image;
        }
      );

      devShells = forEachSystem (
        { system }:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [ pkgs.go_1_27 ];
            env.CGO_ENABLED = "0";
          };
        }
      );

      formatter = forEachSystem ({ system }: nixpkgs.legacyPackages.${system}.nixfmt);
    };
}
