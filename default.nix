let
  _pkgs = import <nixpkgs> {};
in
{ pkgs ? import (_pkgs.fetchFromGitHub { owner = "NixOS";
                                         repo = "nixpkgs-channels";
                                         rev = "7701cbca6b55eb9dee6e61766376dba42a8b32f2";
                                         sha256 = "1f3rix2nkkby8qw7vsafwx0xr84mb7v0186m1hk31w2q09x2s2q8";
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
