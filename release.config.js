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
          }
        ]
      }
    ],
    [
      "@semantic-release/git",
      {
        "assets": ["CHANGELOG.md", "manifests/production/metacontroller.yaml"]
      }
    ],
    [
      '@semantic-release/github', 
      {
        "assets": ["manifests/production/*"]
      }
    ],
  ]
}
