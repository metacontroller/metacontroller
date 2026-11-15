# REVIEW.md

Instructions for automated code review of this repository.

## Use AGENTS.md as the source of the rules

All rules of this repository are in `AGENTS.md` and in the guides in
`.agents/`. Do not use other copies of the rules.

1. Read `AGENTS.md`.
2. Use the table in its section GUIDE INDEX. Read each guide whose
   "Read it when" column applies to this pull request. Compare each
   condition with the changed files, the commit messages, the pull request
   title, and the failed checks. Read only those guides.
3. Check the change against the sections HARD RULES, WRITING STYLE, and DONE
   MEANS of `AGENTS.md`, and against the guides that you read.
4. Name the file and the section of the rule in each finding.

## Check a dependency at its pinned version

Before you report a wrong use of a library, read the source of that library
at the version that the build uses. Do not use the behavior of an older or a
newer version. A library can apply a behavior itself that older code applied
by hand.

To find that version:

1. Find the Go module of the changed file. There are two modules: the root
   and `test/integration`.
2. In the `go.mod` file of that module, use the `replace` directive for the
   library if there is one. Use the `require` line only when there is no
   `replace` directive.

## Out of scope

Tools or the release job write these files and lines. Do not review their
content:

- Whole files that tools generate: `zz_generated.deepcopy.go`, the files in
  `pkg/client/generated/`, and `metacontroller-crds-v1.yaml` in
  `manifests/production/` and in `deploy/helm/metacontroller/crds/`.
- The whole file `CHANGELOG.md`.
- Only the lines `version` and `appVersion` in
  `deploy/helm/metacontroller/Chart.yaml`. Review the other lines.
- Only the image tag in `manifests/production/metacontroller.yaml`. Review
  the other lines.

Do report a pull request that changes one of these files or lines by hand,
as HARD RULES in `AGENTS.md` says. A regenerated file is correct when the
pull request also changes its source.
