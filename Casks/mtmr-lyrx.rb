cask "mtmr-lyrx" do
  version "0.1.0"
  sha256 "2f1695b67d2ada64922005fb2bd9815dcdb029eb4ae3cbea681e46e8ff0c3ee7"

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
