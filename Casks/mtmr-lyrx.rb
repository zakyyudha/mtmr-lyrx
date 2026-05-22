cask "mtmr-lyrx" do
  version "0.1.0"
  sha256 "6c8b335fff8ea75346913a664314ba2b63d63ab4a1554a1f6f215782803bf8e8"

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
