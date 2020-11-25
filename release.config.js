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
            "files": ["manifests/metacontroller.yaml"],
            "from": "image: metacontrollerio/metacontroller:\".*\"",
            "to": "image: metacontrollerio/metacontroller:\"${nextRelease.version}\"",
            "results": [
              {
                "file": "foo/__init__.py",
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
        "assets": ["CHANGELOG.md", "manifests/metacontroller.yaml"]
      }
    ],
    [
      '@semantic-release/github', 
      {
        "assets": ["manifests/*"]
      }
    ],
  ]
}
