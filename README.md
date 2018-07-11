# Tencent Cloud Builder for Packer

This builder plugin creates new images in Tencent Cloud by provisioning a new instance and snapshotting the resulting volume.

## Install

This is assuming you have your `$GOPATH` set properly, or are okay with what modern versions of Go determine it should be by default.

```
go get github.ol.epicgames.net/evan-kinney/packer-builder-tencloud
cp $GOPATH/bin/packer-builder-tencloud /path/to/your/packer
```

`/path/to/your/packer` should be the path to the directory that contains your `packer` binaries.
