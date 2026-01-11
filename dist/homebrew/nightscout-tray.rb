# typed: false
# frozen_string_literal: true

# Homebrew formula for Nightscout Tray
# To install: brew install mrcode/tap/nightscout-tray
class NightscoutTray < Formula
  desc "Nightscout glucose monitoring tray application"
  homepage "https://github.com/mrcode/nightscout-tray"
  license "MIT"
  version "0.0.0" # Updated automatically by CI

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mrcode/nightscout-tray/releases/download/v#{version}/nightscout-tray-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    else
      url "https://github.com/mrcode/nightscout-tray/releases/download/v#{version}/nightscout-tray-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mrcode/nightscout-tray/releases/download/v#{version}/nightscout-tray-linux-arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    else
      url "https://github.com/mrcode/nightscout-tray/releases/download/v#{version}/nightscout-tray-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end

    depends_on "gtk+3"
    depends_on "webkit2gtk"
    depends_on "libappindicator-gtk3" => :recommended
  end

  def install
    bin.install "nightscout-tray"
    
    # Install desktop file on Linux
    if OS.linux?
      (share/"applications").install "nightscout-tray.desktop" if File.exist?("nightscout-tray.desktop")
      (share/"icons/hicolor/256x256/apps").install "nightscout-tray.png" if File.exist?("nightscout-tray.png")
    end
  end

  def post_install
    if OS.mac?
      ohai "To allow Nightscout Tray to run, you may need to:"
      ohai "  1. Right-click the app and select 'Open' the first time"
      ohai "  2. Or go to System Preferences > Security & Privacy to allow it"
    end
  end

  def caveats
    <<~EOS
      Nightscout Tray has been installed!
      
      To start the application, run:
        nightscout-tray

      On first run, you'll need to configure your Nightscout URL in the settings.

      For auto-start on login:
        - Linux: The app can configure this in Settings
        - macOS: Add to Login Items in System Preferences
    EOS
  end

  test do
    assert_match "nightscout-tray", shell_output("#{bin}/nightscout-tray --version 2>&1", 1)
  end
end
