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
            "from": "appVersion: v.*",
            "to": "appVersion: v${nextRelease.version}",
            "results": [
              {
                "file": "deploy/helm/metacontroller/Chart.yaml",
                "hasChanged": true,
                "numMatches": 1,
                "numReplacements": 1
              }
            ],
            "countMatches": true
          },
          {
            "files": ["deploy/helm/metacontroller/Chart.yaml"],
            "from": "version: .*",
            "to": "version: ${nextRelease.version}",
            "results": [
              {
                "file": "deploy/helm/metacontroller/Chart.yaml",
                "hasChanged": true,
                "numMatches": 1,
                "numReplacements": 1
              }
            ],
            "countMatches": true
          },
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
    "@semantic-release/github",
  ],
  // goreleaser (invoked via @semantic-release/exec above) already creates the
  // GitHub Release, so keep @semantic-release/github out of the "publish" step
  // to avoid creating a duplicate release. It still runs its other steps
  // (verifyConditions, success, fail), which is what comments on merged PRs
  // and closed issues with the version that released the fix.
  "publish": [
    "@semantic-release/exec",
  ],
}
