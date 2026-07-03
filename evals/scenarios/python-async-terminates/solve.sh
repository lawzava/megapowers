#!/usr/bin/env bash
command -v python3 >/dev/null 2>&1 || { echo "PY_ABSENT"; exit 0; }
cat > wp.py <<'PY'
import asyncio
async def worker_pool(items, work, concurrency=8):
    sem = asyncio.Semaphore(concurrency)
    async def run(item):
        async with sem:
            return await work(item)
    return await asyncio.gather(*(run(i) for i in items))
async def dbl(x):
    await asyncio.sleep(0.001); return x*2
async def main():
    r = await worker_pool(range(1,6), dbl, concurrency=3)
    print("SUM=%d" % sum(r))
asyncio.run(main())
PY
timeout 30 python3 wp.py > run.out 2>&1 || echo "RUN_TIMEOUT_OR_ERROR" >> run.out
cat run.out
