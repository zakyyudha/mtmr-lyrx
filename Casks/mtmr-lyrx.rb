cask "mtmr-lyrx" do
  version "0.1.0"
  sha256 "f8066e6aec2f89b2eb3e1de45bc707759b28d19a5f2fa6f02879be6db22982ce"

  url "https://github.com/zakyyudha/mtmr-lyrx/releases/download/v#{version}/MTMRLyrx-#{version}-macos.zip"
  name "MTMRLyrx"
  desc "Show synced Spotify lyrics on the MTMR Touch Bar"
  homepage "https://github.com/zakyyudha/mtmr-lyrx"

  depends_on macos: ">= :ventura"

  app "MTMRLyrx.app"
  binary "mtmr-lyrx"

  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/MTMRLyrx.app"],
                   sudo: false
  end

  zap trash: [
    "~/.config/mtmr-lyrx",
  ]
end
