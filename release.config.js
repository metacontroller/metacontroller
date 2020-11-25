module.exports = {
  "branches": ["master"],
  "plugins": [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    '@semantic-release/git',
    ['@semantic-release/github', {
      "assets": ["manifests/*"]
      }],
  ]
}
