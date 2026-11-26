#include "layoutfactory.h"

#include "collectionlayout.h"
#include "decklayout.h"
#include "depthlayout.h"
#include "duellayout.h"
#include "handlayout.h"
#include "hexlayout.h"
#include "sandylayout.h"
#include "sliceslayout.h"
#include "walllayout.h"

std::unique_ptr<Layout> makeLayout(const QString &mode)
{
    if (mode == QLatin1String("depth"))
        return std::make_unique<DepthLayout>();
    if (mode == QLatin1String("wall") || mode == QLatin1String("grid"))
        return std::make_unique<WallLayout>();
    if (mode == QLatin1String("hex"))
        return std::make_unique<HexLayout>();
    if (mode == QLatin1String("sandy") || mode == QLatin1String("nova"))
        return std::make_unique<SandyLayout>();
    if (mode == QLatin1String("hand"))
        return std::make_unique<HandLayout>();
    if (mode == QLatin1String("deck"))
        return std::make_unique<DeckLayout>();
    if (mode == QLatin1String("duel"))
        return std::make_unique<DuelLayout>();
    if (mode == QLatin1String("collection"))
        return std::make_unique<CollectionLayout>();
    return std::make_unique<SlicesLayout>();
}
