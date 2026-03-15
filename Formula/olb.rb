# OpenLoadBalancer Homebrew Formula
#
# Install:
#   brew tap openloadbalancer/olb https://github.com/openloadbalancer/homebrew-olb
#   brew install olb
#
# Or install directly from this repository:
#   brew install --formula Formula/olb.rb

class Olb < Formula
  desc "High-performance zero-dependency load balancer"
  homepage "https://openloadbalancer.dev"
  url "https://github.com/openloadbalancer/olb/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "PLACEHOLDER_SHA256"
  license "Apache-2.0"
  head "https://github.com/openloadbalancer/olb.git", branch: "main"

  depends_on "go" => :build

  def install
    # Resolve version information
    version_pkg = "github.com/openloadbalancer/olb/pkg/version"
    commit = Utils.git_head || "unknown"
    short_commit = commit[0..6]
    build_date = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")

    ldflags = %W[
      -s -w
      -X #{version_pkg}.Version=#{version}
      -X #{version_pkg}.Commit=#{commit}
      -X #{version_pkg}.ShortCommit=#{short_commit}
      -X #{version_pkg}.Date=#{build_date}
    ]

    system "go", "build",
           *std_go_args(ldflags:, output: bin/"olb"),
           "-trimpath",
           "./cmd/olb"

    # Install default configuration
    (etc/"olb").mkpath
    (etc/"olb").install "configs/olb.yaml" => "olb.yaml.default" unless (etc/"olb/olb.yaml").exist?
    (etc/"olb").install "configs/olb.yaml" unless (etc/"olb/olb.yaml").exist?

    # Install shell completions if available
    generate_completions_from_executable(bin/"olb", "completion", shells: []) if respond_to?(:generate_completions_from_executable)
  end

  def post_install
    # Create log directory
    (var/"log/olb").mkpath

    # Create data directory (certificates, state, etc.)
    (var/"lib/olb").mkpath
  end

  service do
    run [opt_bin/"olb", "start", "--config", etc/"olb/olb.yaml"]
    keep_alive true
    working_dir var/"lib/olb"
    log_path var/"log/olb/olb.log"
    error_log_path var/"log/olb/olb.err.log"
    environment_variables GOMAXPROCS: "0"
  end

  def caveats
    <<~EOS
      OpenLoadBalancer has been installed.

      Configuration:
        Default config:  #{etc}/olb/olb.yaml.default
        Active config:   #{etc}/olb/olb.yaml
        Edit config:     $EDITOR #{etc}/olb/olb.yaml

      Directories:
        Logs:            #{var}/log/olb/
        Data:            #{var}/lib/olb/

      Quick start:
        1. Edit the configuration file:
             $EDITOR #{etc}/olb/olb.yaml

        2. Start manually:
             olb start --config #{etc}/olb/olb.yaml

        3. Or start as a background service:
             brew services start olb

        4. Check status:
             olb status

        5. View live dashboard:
             olb top

      Admin API (default):
        http://127.0.0.1:8081

      To stop the service:
        brew services stop olb
    EOS
  end

  test do
    # Verify the binary runs and reports the correct version
    assert_match version.to_s, shell_output("#{bin}/olb version")

    # Verify the binary can generate a default config
    system bin/"olb", "init", "--dir", testpath
    assert_predicate testpath/"olb.yaml", :exist?

    # Verify the binary can validate a config (health check with no server running)
    output = shell_output("#{bin}/olb health 2>&1", 1)
    assert_match(/connection refused|cannot connect|error/i, output)
  end
end
