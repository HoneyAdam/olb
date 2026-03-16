#!/bin/sh
# Redirect to the actual install script from the repo
exec curl -fsSL https://raw.githubusercontent.com/openloadbalancer/olb/main/scripts/install.sh | sh -s -- "$@"
