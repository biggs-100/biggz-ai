class BiggzAi < Formula
  desc "AI Agent Harness — Review-Driven Development with Human-in-the-Loop"
  homepage "https://github.com/biggs-100/biggz-ai"
  license "MIT"
  version "1.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_darwin_arm64.tar.gz"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
    else
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_darwin_amd64.tar.gz"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_linux_arm64.tar.gz"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
    else
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_linux_amd64.tar.gz"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
    end
  end

  on_windows do
    if Hardware::CPU.arm?
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_windows_arm64.zip"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
    else
      url "https://github.com/biggs-100/biggz-ai/releases/download/v1.0.0/biggz_windows_amd64.zip"
      sha256 "UPDATE_ME_AFTER_GORELEASER_RUNS"
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
