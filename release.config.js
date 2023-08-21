module.exports = {
  "branches": ["master"],
  "plugins": [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    [
      "@semantic-release/changelog",
      {
        "changelogFile": "CHANGELOG.md"
      }
    ],
    [
      "@google/semantic-release-replace-plugin",
      {
        "replacements": [
          {
            "files": ["manifests/production/metacontroller.yaml"],
            "from": "metacontroller:v.*",
            "to": "metacontroller:v${nextRelease.version}",
            "results": [
              {
                "file": "manifests/production/metacontroller.yaml",
                "hasChanged": true,
                "numMatches": 1,
                "numReplacements": 1
              }
            ],
            "countMatches": true
          },
          {
            "files": ["deploy/helm/metacontroller/Chart.yaml"],
            "from": "(version|appVersion): v.*",
            "to": "$1: v${nextRelease.version}",
            "results": [
              {
                "file": "deploy/helm/metacontroller/Chart.yaml",
                "hasChanged": true,
                "numMatches": 2,
                "numReplacements": 2
              }
            ],
            "countMatches": true
          }
        ]
      }
    ],
    [
      "@semantic-release/git",
      {
        "assets": ["CHANGELOG.md", "manifests/production/metacontroller.yaml", "deploy/helm/metacontroller/Chart.yaml"],
        "message": "chore(release): ${nextRelease.version}\n\n${nextRelease.notes}"
      }
    ],
    ["@semantic-release/exec",
      {
        "publishCmd": "./.release.sh \"${nextRelease.notes}\""
      }
    ],
  ]
}
