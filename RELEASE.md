## Version

* Use Semantic Versioning: `vMAJOR.MINOR.PATCH`

---

## Pre-release Checklist

* [ ] Checkout to the release branch
* [ ] Run tests and ensure all tests pass
* [ ] Update `CHANGELOG.md` based on commit history since the previous release (release version is determined by the current CHANGELOG). If this is the first release, get all commit history since first commit (using `git log`).
* [ ] Ensure the working directory is clean

---

## Release Steps

1. Create release branch

```bash
git checkout -b release/vX.Y.Z
```

2. Update `CHANGELOG.md`
   
3. Commit

```bash
git add .
git commit -m "release: vX.Y.Z"
```

5. Push branch

```bash
git push origin release/vX.Y.Z
```

6. Create a PR into `master`

7. After PR is merged into `master`, create tag:

```bash
git checkout master
git pull
git tag vX.Y.Z
git push origin vX.Y.Z
```

---

## Rules

* Do not release if anything is outdated or failing
* Do not rewrite history after release
