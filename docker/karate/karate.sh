#!/bin/bash

set -e

### DO NOT alter any paths in this section ###
KARATE_JAR="/app/karate.jar"
##############################################

# User-specified CLI args
test_path="${KARATE_CONFIG_MODULE_ROOT}/test/karate/features"
config_dir="${KARATE_CONFIG_MODULE_ROOT}/test/karate"
karate_env="docker"
output_dir="/app/target"

# Build the Karate CLI args
karate_command=()
karate_command+=(-Dkarate.config.dir="${config_dir}")
karate_command+=("${KARATE_JAR}")
karate_command+=("${test_path}")
karate_command+=("-e" "${karate_env}")
karate_command+=("-o" "${output_dir}")
karate_command+=("--tags" "~@ignore")

# Run Karate with args
echo "🥋 Running Karate with args: java -jar ${karate_command[*]}"
java -jar ${karate_command[*]}
