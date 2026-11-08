#include "buslistdiff.hpp"

#include <QHash>
#include <QSet>

#include <algorithm>

namespace buslistdiff {

namespace {

// longest strictly increasing subsequence, as indices into seq
QList<int> longestRun(const QList<int> &seq)
{
    QList<int> tails;  // index into seq of the smallest tail of each length
    QList<int> prev(seq.size(), -1);
    for (int i = 0; i < seq.size(); ++i) {
        const auto at = std::lower_bound(tails.begin(), tails.end(), seq.at(i),
                                         [&](int t, int v) { return seq.at(t) < v; });
        const int len = int(at - tails.begin());
        if (len > 0)
            prev[i] = tails.at(len - 1);
        if (at == tails.end())
            tails.append(i);
        else
            *at = i;
    }
    QList<int> out(tails.size());
    int k = tails.isEmpty() ? -1 : tails.last();
    for (qsizetype n = out.size() - 1; n >= 0; --n) {
        out[n] = k;
        k = prev.at(k);
    }
    return out;
}

}

QList<Op> diff(const QStringList &before, const QStringList &after)
{
    QList<Op> ops;
    if (before == after)
        return ops;

    QHash<QString, int> wanted;
    wanted.reserve(after.size());
    for (int i = 0; i < after.size(); ++i)
        wanted.insert(after.at(i), i);

    QStringList cur;
    cur.reserve(before.size());
    for (int i = int(before.size()) - 1; i >= 0;) {
        if (wanted.contains(before.at(i))) {
            --i;
            continue;
        }
        const int end = i;
        while (i >= 0 && !wanted.contains(before.at(i)))
            --i;
        ops.append({Op::Remove, i + 1, end - i, 0, 0});
    }
    for (const QString &k : before)
        if (wanted.contains(k))
            cur.append(k);

    QHash<QString, int> kept;
    kept.reserve(cur.size());
    for (int i = 0; i < cur.size(); ++i)
        kept.insert(cur.at(i), i);

    QList<int> seq;
    QStringList seqKeys;
    for (const QString &k : after) {
        const auto it = kept.constFind(k);
        if (it != kept.constEnd()) {
            seq.append(it.value());
            seqKeys.append(k);
        }
    }
    QSet<QString> stable;
    for (int i : longestRun(seq))
        stable.insert(seqKeys.at(i));

    for (int i = 0; i < after.size(); ++i) {
        const QString &k = after.at(i);
        for (;;) {
            if (i < cur.size() && cur.at(i) == k)
                break;
            if (!kept.contains(k)) {
                int n = 1;
                while (i + n < after.size() && !kept.contains(after.at(i + n)))
                    ++n;
                ops.append({Op::Insert, i, n, 0, i});
                for (int m = 0; m < n; ++m)
                    cur.insert(i + m, after.at(i + m));
                i += n - 1;
                break;
            }
            const int j = int(cur.indexOf(k, i + 1));
            if (j < 0)
                return ops;  // unreachable with unique keys
            // k stays on the stable run so the row in its way left it, park that row at the bottom
            if (stable.contains(k) && !stable.contains(cur.at(i))) {
                const int last = int(cur.size()) - 1;
                ops.append({Op::Move, i, 1, last, 0});
                cur.move(i, last);
                continue;
            }
            ops.append({Op::Move, j, 1, i, 0});
            cur.move(j, i);
            break;
        }
    }
    return ops;
}

QStringList replay(QStringList before, const QStringList &after, const QList<Op> &ops)
{
    for (const Op &op : ops) {
        switch (op.kind) {
        case Op::Remove:
            before.remove(op.row, op.count);
            break;
        case Op::Insert:
            for (int m = 0; m < op.count; ++m)
                before.insert(op.row + m, after.at(op.source + m));
            break;
        case Op::Move:
            before.move(op.row, op.to);
            break;
        }
    }
    return before;
}

}
