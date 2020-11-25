module.exports = {
  "branches": ["master"],
  "plugins": [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    '@semantic-release/github', {
      "assets": {"path": "manifests/metacontroller.yaml", "label": "Manifests"},
      "addReleases": "top"
    }]
}
