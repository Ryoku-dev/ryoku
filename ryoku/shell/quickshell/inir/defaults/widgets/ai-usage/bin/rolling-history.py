"""Build period totals and chart bins from the same trailing 24-hour sample."""

HOUR_MS = 3600000


def rolling_period(messages, now_ms, summarize):
    end = int(now_ms)
    start = end - 24 * HOUR_MS
    buckets = [dict(start=start + index * HOUR_MS,
                    end=start + (index + 1) * HOUR_MS, tokens=0, cacheRead=0)
               for index in range(24)]
    def selected():
        for message in messages:
            stamp = message["time"]
            if not start <= stamp <= end:
                continue
            bucket = buckets[min(23, int((stamp - start) // HOUR_MS))]
            bucket["tokens"] += message["total"]
            bucket["cacheRead"] += message["cache_read"]
            yield message
    result = summarize(selected(), start)
    result.update(start=start, end=end, hours=buckets)
    return result
