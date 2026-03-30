{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  buildInputs = [
    pkgs.glibcLocales

    pkgs.zip
    pkgs.unzip
    pkgs.tzdata
    pkgs.xclip

    pkgs.enca
    pkgs.bc
    pkgs.yq
    pkgs.colorized-logs # ansi2txt

    pkgs.ffmpeg
    pkgs.imagemagick
    # pkgs.util-linux

    pkgs.gh
    pkgs.zenith
    pkgs.git-lfs
    pkgs.lazygit

    pkgs.kubectl
    pkgs.kubectx
    pkgs.kubernetes-helm
    # pkgs.skopeo

    # pkgs.docker
    # pkgs.docker-compose
    # pkgs.docker-buildx

    # emacs
    # pkgs.pipenv # conflict with the one installed by asdf
    # pkgs.terraform # install from pre-compiled binary
    pkgs.emacs30
    pkgs.libtool
    pkgs.cmake
    pkgs.fontconfig

    # pkgs.nodejs_22
    # pkgs.ruby
    # pkgs.rubyPackages.solargraph
    # pkgs.rufo
    # pkgs.rubocop

    pkgs.python311Packages.python
    pkgs.python311Packages.pyflakes
    pkgs.python311Packages.pytest

    # failed to build due to some missing dependency
    # pkgs.python311Packages.isort
    # pkgs.python311Packages.black

    # pkgs.jdk21
    # pkgs.dockfmt
    pkgs.nixfmt-classic
    pkgs.shellcheck
    pkgs.shfmt
    pkgs.ispell
    pkgs.clang-tools
    pkgs.gotests
    pkgs.gopls
    pkgs.gomodifytags
    pkgs.gore
    pkgs.gotools

    # pkgs.go
    # pkgs.ruby
    pkgs.python311Packages.python

    pkgs.google-cloud-sdk

    pkgs.ruff
    pkgs.nodePackages.prettier
    pkgs.pandoc

    # for cp.jsf
    # pkgs.awscli2
  ];
  LOCALE_ARCHIVE = "${pkgs.glibcLocales}/lib/locale/locale-archive";
}
