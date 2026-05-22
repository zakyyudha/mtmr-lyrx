// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MTMRLyrx",
    platforms: [
        .macOS(.v13)
    ],
    targets: [
        .executableTarget(
            name: "MTMRLyrx",
            path: "Sources/MTMRLyrxMenu",
            resources: [
                .process("Resources")
            ]
        )
    ]
)
