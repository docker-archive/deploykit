let
  _pkgs = import <nixpkgs> {};
in
{ pkgs ? import (_pkgs.fetchFromGitHub { owner = "NixOS";
                                         repo = "nixpkgs-channels";
                                         rev = "0afb6d789c8bf74825e8cdf6a5d3b9ab8bde4f2d";
                                         sha256 = "147vhzrnwcy0v77kgbap31698qbda8rn09n5fnjp740svmkjpaiz";
                                       }) {}
}:

pkgs.stdenv.mkDerivation rec {
  name = "docker-pipeline-dev";
  env = pkgs.buildEnv { name = name; paths = buildInputs; };
  buildInputs = [
    pkgs.vndr
    pkgs.docker
    pkgs.gnumake
    pkgs.go_1_8
    pkgs.gcc
    pkgs.gotools
  ];
}
