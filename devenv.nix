# SPDX-FileCopyrightText: 2026 tao3k team and Contributors
#
# SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

{
  pkgs,
  ...
}:
{
  packages = [
    pkgs.buf
    pkgs.bun
    pkgs.just
    pkgs.protobuf
    pkgs.protoc-gen-go
    pkgs.protoc-gen-go-grpc
    pkgs.reuse
  ];

  languages.go = {
    enable = true;
  };

  dotenv.enable = true;

  git-hooks.hooks = {
    nixfmt.enable = true;
  };
}
