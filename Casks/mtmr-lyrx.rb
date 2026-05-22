cask "mtmr-lyrx" do
  version "0.1.0"
  sha256 "ba893e2acaee2ae7a8896d2f5a2d2f344a108ab4525e412c227c8d097eb297f5"

  url "https://github.com/zakyyudha/mtmr-lyrx/releases/download/v#{version}/MTMRLyrx-#{version}-macos.zip"
  name "MTMRLyrx"
  desc "Show synced Spotify lyrics on the MTMR Touch Bar"
  homepage "https://github.com/zakyyudha/mtmr-lyrx"

  depends_on macos: ">= :ventura"

  app "MTMRLyrx.app"
  binary "mtmr-lyrx"

  zap trash: [
    "~/.config/mtmr-lyrx",
  ]
end
