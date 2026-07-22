const githubLoginCache = new Map();

const execPlugin = [
  "@semantic-release/exec",
  {
    "publishCmd": "./.release.sh \"${nextRelease.notes}\""
  }
];

async function resolveGithubLogin(commit) {
  const cacheKey = commit.author.email;
  if (githubLoginCache.has(cacheKey)) return githubLoginCache.get(cacheKey);

  let login = null;
  try {
    const res = await fetch(
      `https://api.github.com/repos/metacontroller/metacontroller/commits/${commit.hash}`,
      { headers: { Authorization: `Bearer ${process.env.GH_TOKEN}` } }
    );
    if (res.ok) {
      const data = await res.json();
      login = data.author?.login ?? null;
    }
  } catch {
    // network/API hiccup - fall back to git author name below
  }

  githubLoginCache.set(cacheKey, login);
  return login;
}

// conventional-changelog-angular is ESM-only, so it's loaded via dynamic
// import() (works fine from this CommonJS file) instead of require(). This
// keeps the changelog's type grouping, breaking-change notes and filtering
// of non-release commits (chore/ci/style/...) identical to the default
// angular preset - we only layer `githubLogin` on top of what it returns.
let angularTransform;
async function getAngularTransform() {
  if (!angularTransform) {
    try {
      const { default: createPreset } = await import("conventional-changelog-angular");
      angularTransform = createPreset().writer.transform;
    } catch (err) {
      throw new Error(`Failed to load conventional-changelog-angular: ${err.message}`);
    }
  }
  return angularTransform;
}

module.exports = {
  "branches": ["master"],
  "plugins": [
    '@semantic-release/commit-analyzer',
    [
      "@semantic-release/release-notes-generator",
      {
        writerOpts: {
          transform: async (commit, context) => {
            const transform = await getAngularTransform();
            const patch = await transform(commit, context);
            // angular's transform returns undefined for commits it excludes
            // from the changelog (chores, ci, docs, ...) - keep that intact.
            if (!patch) return patch;
            // angular's transform only returns {notes, type, scope, shortHash,
            // subject, references} - it drops `hash` and `author`, which
            // commitPartial also needs. Spread the original commit first so
            // those survive, then layer patch's processed values on top.
            return { ...commit, ...patch, githubLogin: await resolveGithubLogin(commit) };
          },
          // Same as conventional-changelog-angular@8.3.1's default commitPartial,
          // with " — @login" (or the git author name as fallback) appended.
          // NOTE: if the dependency is upgraded, verify this template is still in sync.
          commitPartial: `*{{#if scope}} **{{scope}}:**
{{~/if}} {{#if subject}}
  {{~subject}}
{{~else}}
  {{~header}}
{{~/if}}

{{~!-- commit link --}} {{#if @root.linkReferences~}}
  ([{{shortHash}}](
  {{~#if @root.repository}}
    {{~#if @root.host}}
      {{~@root.host}}/
    {{~/if}}
    {{~#if @root.owner}}
      {{~@root.owner}}/
    {{~/if}}
    {{~@root.repository}}
  {{~else}}
    {{~@root.repoUrl}}
  {{~/if}}/
  {{~@root.commit}}/{{hash}}))
{{~else}}
  {{~shortHash}}
{{~/if}}

{{~!-- commit references --}}
{{~#if references~}}
  , closes
  {{~#each references}} {{#if @root.linkReferences~}}
    [
    {{~#if this.owner}}
      {{~this.owner}}/
    {{~/if}}
    {{~this.repository}}#{{this.issue}}](
    {{~#if @root.repository}}
      {{~#if @root.host}}
        {{~@root.host}}/
      {{~/if}}
      {{~#if this.repository}}
        {{~#if this.owner}}
          {{~this.owner}}/
        {{~/if}}
        {{~this.repository}}
      {{~else}}
        {{~#if @root.owner}}
          {{~@root.owner}}/
        {{~/if}}
          {{~@root.repository}}
        {{~/if}}
    {{~else}}
      {{~@root.repoUrl}}
    {{~/if}}/
    {{~@root.issue}}/{{this.issue}})
  {{~else}}
    {{~#if this.owner}}
      {{~this.owner}}/
    {{~/if}}
    {{~this.repository}}#{{this.issue}}
  {{~/if}}{{/each}}
{{~/if}} — {{#if githubLogin}}@{{githubLogin}}{{else}}{{author.name}}{{/if}}

`,
        },
      },
    ],
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
    execPlugin,
    "@semantic-release/github",
  ],
  // goreleaser (invoked via @semantic-release/exec above) already creates the
  // GitHub Release, so keep @semantic-release/github out of the "publish" step
  // to avoid creating a duplicate release. It still runs its other steps
  // (verifyConditions, success, fail), which is what comments on merged PRs
  // and closed issues with the version that released the fix.
  //
  // NOTE: the plugin must be repeated here as the same [name, config] tuple
  // (not just "@semantic-release/exec") - referencing it by bare name drops
  // the publishCmd option, silently turning this step into a no-op and
  // skipping goreleaser entirely (see v4.16.3, which shipped a tag/commit but
  // no Docker images or GitHub release).
  "publish": [
    execPlugin,
  ],
}
