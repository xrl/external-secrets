#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Validate fork-only image identities and exact native manifest membership."""
import argparse
import json
import re

REPOSITORY = "ghcr.io/xrl/external-secrets"
ARCHES = ("amd64", "arm64")


def tag(run_id, attempt, revision):
    if not re.fullmatch(r"[1-9][0-9]*", run_id) or not re.fullmatch(r"[1-9][0-9]*", attempt):
        raise ValueError("run ID and attempt must be positive integers")
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("revision must be a full lowercase Git SHA")
    return f"v2.9.0-xrl.{run_id}.{attempt}-g{revision[:12]}"


def publish_allowed(repository, ref, event):
    return repository == "xrl/external-secrets" and ref == "refs/heads/xrl/integration" and event in ("push", "workflow_dispatch")


def validate_tag(value):
    if not re.fullmatch(r"v2\.9\.0-xrl\.[1-9][0-9]*\.[1-9][0-9]*-g[0-9a-f]{12}", value):
        raise ValueError("not a unique fork tag")


def validate_digest(value):
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", value):
        raise ValueError("invalid image digest")


def references(directory, version):
    validate_tag(version)
    refs = []
    for arch in ARCHES:
        with open(f"{directory}/{arch}.json", encoding="utf-8") as handle:
            record = json.load(handle)
        if record["image"] != f"{REPOSITORY}:{version}-{arch}" or record["architecture"] != arch:
            raise ValueError("unexpected child image identity")
        validate_digest(record["digest"])
        refs.append(f"{REPOSITORY}@{record['digest']}")
    if refs[0] == refs[1]:
        raise ValueError("architectures must have distinct child digests")
    return refs


def validate_manifest(manifest, refs):
    descriptors = manifest.get("manifests", [])
    expected = dict(zip(ARCHES, (ref.split("@")[1] for ref in refs)))
    if len(descriptors) != 2:
        raise ValueError("manifest must contain exactly two native images")
    seen = set()
    for item in descriptors:
        platform = item["platform"]
        arch = platform["architecture"]
        if platform["os"] != "linux" or arch in seen or expected.get(arch) != item["digest"]:
            raise ValueError("manifest platform/digest mismatch")
        seen.add(arch)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    tags = sub.add_parser("tag")
    tags.add_argument("run_id")
    tags.add_argument("attempt")
    tags.add_argument("revision")
    guard = sub.add_parser("guard")
    guard.add_argument("repository")
    guard.add_argument("ref")
    guard.add_argument("event")
    record = sub.add_parser("record")
    record.add_argument("version")
    record.add_argument("architecture", choices=ARCHES)
    record.add_argument("digest")
    for command in ("references", "verify"):
        command_parser = sub.add_parser(command)
        command_parser.add_argument("directory")
        command_parser.add_argument("version")
        if command == "verify":
            command_parser.add_argument("manifest")
    args = parser.parse_args()
    if args.command == "tag":
        print(tag(args.run_id, args.attempt, args.revision))
    elif args.command == "guard":
        if not publish_allowed(args.repository, args.ref, args.event):
            raise ValueError("publishing is restricted to the exact fork integration ref")
    elif args.command == "record":
        validate_tag(args.version)
        validate_digest(args.digest)
        print(json.dumps({"image": f"{REPOSITORY}:{args.version}-{args.architecture}", "architecture": args.architecture, "digest": args.digest}))
    else:
        refs = references(args.directory, args.version)
        if args.command == "references":
            print("\n".join(refs))
        else:
            with open(args.manifest, encoding="utf-8") as handle:
                validate_manifest(json.load(handle), refs)


if __name__ == "__main__":
    main()
