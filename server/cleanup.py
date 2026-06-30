import asyncio
import argparse
from datetime import datetime, timedelta
from database import init_db, get_db, close_db


async def cleanup_orphans(max_age_hours: float = 24, simulate: bool = False):
    await init_db()
    db = await get_db()

    cutoff = (datetime.utcnow() - timedelta(hours=max_age_hours)).isoformat()

    rows = await db.execute(
        "SELECT id, name, hostname, registered_at, last_seen FROM machines "
        "WHERE (last_seen IS NULL AND registered_at < ?) OR last_seen < ?",
        (cutoff, cutoff),
    )
    orphans = await rows.fetchall()

    if not orphans:
        print("No orphan machines found.")
        await close_db()
        return

    print(f"Found {len(orphans)} orphan machine(s) (no activity > {max_age_hours}h):")
    for m in orphans:
        print(f"  {m['id']:14s} {m['name']:20s} last_seen={m['last_seen'] or 'never'}")

    if simulate:
        print("\n[SIMULATE] Use --commit to actually delete.")
        await close_db()
        return

    print("\nDeleting...")
    for m in orphans:
        await db.execute("DELETE FROM tasks WHERE machine_id=?", (m["id"],))
        await db.execute("DELETE FROM machines WHERE id=?", (m["id"],))
        print(f"  deleted {m['id']} ({m['name']})")

    await db.commit()
    print(f"Done. Removed {len(orphans)} orphans.")

    await close_db()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Cleanup orphan registrations")
    parser.add_argument("--age", type=float, default=24, help="Max age in hours (default: 24)")
    parser.add_argument("--commit", action="store_true", help="Actually delete (dry-run by default)")
    args = parser.parse_args()

    asyncio.run(cleanup_orphans(max_age_hours=args.age, simulate=not args.commit))
