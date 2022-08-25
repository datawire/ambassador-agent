## Publish Mechanism

We use the [publish](.github/workflows/publish.yaml) action to
publish new versions of the extensions whenever a new tag is added to the
repo.

Images are pushed to [ambassador/ambassador-agent](https://hub.docker.com/repository/docker/ambassador/ambassador-agent).
We use multi-arch docker builds since the images need to be supported on
amd64 and arm64 machines, for more information on multi-arch docker builds
you can take a look at this
[dockerpage](https://www.docker.com/blog/multi-arch-build-and-images-the-simple-way/)

To trigger the publish workfow, run the following commands:

```
git tag --annotate --message='Releasing version vSEMVER' vSEMVER
git push origin vSEMVER
```

You can then follow along in the [actions tab](https://github.com/datawire/ambassador-agent/actions)
