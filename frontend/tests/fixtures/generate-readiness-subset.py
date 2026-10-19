"""Extract unchanged authored objects; never publish or manufacture review receipts.

Usage: python3 tests/fixtures/generate-readiness-subset.py /path/to/assembled.json
"""

import hashlib
import json
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).read_bytes()
package = json.loads(source)
ids = {"trc-required", "aar-required", "ksi-ced-rat-required"}
books = [book for book in package["playbooks"] if book["id"] in ids]
assert len(books) == 3
capability_ids = {p["capability_id"] for b in books for p in b["prerequisites"]}
capabilities = []
while capability_ids:
    capability = next(c for c in package["capabilities"] if c["id"] in capability_ids)
    capability_ids.remove(capability["id"])
    capabilities.append(capability)
    for dependency in capability.get("dependencies") or []:
        dependency_id = dependency if isinstance(dependency, str) else dependency["capability_id"]
        if dependency_id not in {c["id"] for c in capabilities}:
            capability_ids.add(dependency_id)

citation_ids = set()


def references(value):
    if isinstance(value, dict):
        citation_ids.update(value.get("citations") or [])
        for child in value.values():
            references(child)
    elif isinstance(value, list):
        for child in value:
            references(child)


references(books)
references(capabilities)
subset = {key: package[key] for key in ("format", "catalog_id", "revision", "sources")}
subset.update(playbooks=books, capabilities=capabilities,
              citations=[c for c in package["citations"] if c["id"] in citation_ids])
assert len(subset["citations"]) == len(citation_ids)
output = (json.dumps(subset, indent=2, ensure_ascii=False) + "\n").encode()
directory = pathlib.Path(__file__).parent
(directory / "readiness-authored-subset.json").write_bytes(output)
manifest = {"assembled_sha256": hashlib.sha256(source).hexdigest(),
            "subset_sha256": hashlib.sha256(output).hexdigest(),
            "selection": sorted(ids), "sources": package["sources"],
            "notice": "Unchanged authored draft subset; no authenticated review receipt. Service evidence is supplied separately and is synthetic."}
(directory / "readiness-authored-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
