module.exports = {
  "branches": [
    { "name": "main" },
    { "name": "beta", "channel": "beta", "prerelease": "beta" },
  ],
  "plugins": [
    "@semantic-release/commit-analyzer",
    "@semantic-release/release-notes-generator",
    ["@semantic-release/exec", {
      "prepareCmd": "make NEXT=${nextRelease.version} update-doc-version"
    }],
    ["@semantic-release/git", {
      "assets": [["docker-compose.yml"]],
      "message": "docs(release): update doc version from ${lastRelease.version} to ${nextRelease.version} [skip ci]"
    }],
    "@semantic-release/github"
  ]
}