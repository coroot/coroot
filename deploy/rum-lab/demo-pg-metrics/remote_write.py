"""Minimal Prometheus remote-write (prompb.WriteRequest) encoder + snappy."""
from __future__ import annotations

import struct
import urllib.error
import urllib.request


def _varint(n: int) -> bytes:
    out = bytearray()
    while True:
        b = n & 0x7F
        n >>= 7
        out.append(b | (0x80 if n else 0))
        if not n:
            break
    return bytes(out)


def _key(field: int, wire: int) -> bytes:
    return _varint((field << 3) | wire)


def _bytes_field(field: int, data: bytes) -> bytes:
    return _key(field, 2) + _varint(len(data)) + data


def _string_field(field: int, s: str) -> bytes:
    return _bytes_field(field, s.encode("utf-8"))


def _double_field(field: int, v: float) -> bytes:
    return _key(field, 1) + struct.pack("<d", v)


def _int64_zigzag_field(field: int, v: int) -> bytes:
    # proto3 int64 as varint (not zigzag) for Sample.timestamp
    if v < 0:
        v += 1 << 64
    return _key(field, 0) + _varint(v)


def encode_label(name: str, value: str) -> bytes:
    return _string_field(1, name) + _string_field(2, value)


def encode_sample(value: float, ts_ms: int) -> bytes:
    return _double_field(1, value) + _int64_zigzag_field(2, ts_ms)


def encode_timeseries(labels: list[tuple[str, str]], samples: list[tuple[float, int]]) -> bytes:
    body = b"".join(_bytes_field(1, encode_label(n, v)) for n, v in labels)
    body += b"".join(_bytes_field(2, encode_sample(val, ts)) for val, ts in samples)
    return body


def encode_write_request(series: list[tuple[list[tuple[str, str]], list[tuple[float, int]]]]) -> bytes:
    return b"".join(_bytes_field(1, encode_timeseries(labels, samples)) for labels, samples in series)


def snappy_compress(data: bytes) -> bytes:
    import snappy  # type: ignore

    return snappy.compress(data)


def remote_write(url: str, api_key: str, series: list[tuple[list[tuple[str, str]], list[tuple[float, int]]]]) -> None:
    body = snappy_compress(encode_write_request(series))
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/x-protobuf",
            "Content-Encoding": "snappy",
            "X-Prometheus-Remote-Write-Version": "0.1.0",
            "X-API-Key": api_key,
        },
    )
    with urllib.request.urlopen(req, timeout=5) as resp:
        resp.read()
