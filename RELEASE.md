## Version

* Use Semantic Versioning: `vMAJOR.MINOR.PATCH`

---

## Pre-release Checklist

* [ ] Run tests and ensure all tests pass
* [ ] Update `CHANGELOG.md` based on commit history since the previous version
* [ ] Update version in `skills/peer/SKILL.md` to the current version
* [ ] Ensure the working directory is clean

---

## Release Steps

1. Create release branch

```bash
git checkout -b release/vX.Y.Z
```

2. Update version

3. Update `CHANGELOG.md` and `skills/peer/SKILL.md`

4. Commit

```bash
git add .
git commit -m "release: vX.Y.Z"
```

5. Push branch

```bash
git push origin release/vX.Y.Z
```

6. Create a PR into `main`

7. After PR is merged into `main`, create tag:

```bash
git checkout main
git pull
git tag vX.Y.Z
git push origin vX.Y.Z
```

---

## Rules

* CLI version and skill must match the same commit
* Do not release if anything is outdated or failing
* Do not rewrite history after release
