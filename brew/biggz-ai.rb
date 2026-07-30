class BiggzAi < Formula
  desc "AI Agent Harness — Review-Driven Development with Human-in-the-Loop"
  homepage "https://github.com/biggs-100/biggz-ai"
  license "MIT"
  version "dev"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/biggs-100/biggz-ai/releases/download/v#{version}/biggz-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/biggs-100/biggz-ai/releases/download/v#{version}/biggz-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/biggs-100/biggz-ai/releases/download/v#{version}/biggz-linux-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/biggs-100/biggz-ai/releases/download/v#{version}/biggz-linux-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    bin.install "biggz"
    bin.install "biggz-mcp"
  end

  test do
    assert_match "Usage:", shell_output("#{bin}/biggz --help")
  end
end
