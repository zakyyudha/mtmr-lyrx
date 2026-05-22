cask "mtmr-lyrx" do
  version "0.1.0"
  sha256 "572a11f80853950b1593f807b03ff5c37c6aa276de3de33c4389ca57cc14f355"

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
