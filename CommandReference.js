cmd(['forge'], withInteraction(withMultiQuery(async (ctx, user, cards, parsedargs) => {
    if(!parsedargs[0] || parsedargs[0].isEmpty())
        return ctx.qhelp(ctx, user, 'forge')

    const batch1 = cards[0].filter(x => !x.locked)
    const batch2 = cards[1]?.filter(x => !x.locked)

    const isExcludedCollection = card => {
        const collection = ctx.collections.find(x => x.id === card.col);
        return collection.fragments || collection.album || collection.liveauction || collection.jackpot || collection.birthdays || collection.limited;
    }

    const batch1_filtered = batch1.filter(x => !isExcludedCollection(x));
    const batch2_filtered = batch2 ? batch2.filter(x => !isExcludedCollection(x)) : undefined;

    let card1, card2

    if(!batch1_filtered || batch1_filtered.length == 0) {
        return ctx.reply(user, `couldn't find any matching cards`, 'red')
    }

    card1 = batch1_filtered[0]

    if(batch2_filtered && batch2_filtered.length > 0) {
        card2 = batch2_filtered.filter(x => x.id != card1.id)[0]
    } else {
        card2 = batch1_filtered.filter(x => x.id != card1.id)[0]
    }

    const conditions = ['lottery', 'fragments', 'album', 'liveauction', 'jackpot', 'birthdays', 'limited'];
    const col1 = ctx.collections.find(x => x.id === card1.col && x.promo)
    const col2 = ctx.collections.find(x => x.id === card2.col && x.promo)

    for (let condition of conditions) {
        if ((col1 && col1[condition]) || (col2 && col2[condition])) {
            return ctx.reply(user, `you cannot forge cards from ${condition} collection`, 'red');
        }
    }

    if(!card1 || !card2)
        return ctx.reply(user, `not enough unique cards found matching this query.
            You can specify one query that can get 2+ unique cards, or 2 queries using \`,\` as separator`, 'red')

    if(card1.level != card2.level)
        return ctx.reply(user, `you can forge only cards of the same star count`, 'red')

    // if(card1.level > 3)
    //     return ctx.reply(user, `you cannot forge cards higher than 3 ${ctx.symbols.star}`, 'red')

    const eval1 = await evalCard(ctx, card1)
    const eval2 = await evalCard(ctx, card2)
    const vialavg = (await getVialCost(ctx, card1, eval1) + await getVialCost(ctx, card2, eval2)) * .1
    const starCostMultiplier = card1.level === 4 ? 7 : 1; // cost multiplier for 4-star cards
    const cost = Math.round(((eval1 + eval2) * .25 * starCostMultiplier) * (await check_effect(ctx, user, 'cherrybloss')? .3 : 1))
    let vialres = Math.round((vialavg === Infinity? 0 : vialavg) * .2)

    if (card1.level === 4) {
        vialres = 0
    }

    if(user.exp < cost)
        return ctx.reply(user, `you need at least **${numFmt(cost)}** ${ctx.symbols.tomato} to forge these cards`, 'red')

    if((card1.fav && card1.amount == 1) || (card2.fav && card2.amount == 1))
        return ctx.reply(user, `your query contains last copy of your favourite card(s). Please remove it from favourites and try again`, 'red')

    const question = `Do you want to forge ${formatName(card1)}**(x${card1.amount})** and ${formatName(card2)}**(x${card2.amount})** using **${numFmt(cost)}** ${ctx.symbols.tomato}?
        You will get **${numFmt(vialres)}** ${ctx.symbols.vial} and a **${card1.level} ${ctx.symbols.star} card**`


    // Check if both cards are promo and from the same collection
    const isSameCollectionPromo = (card1, card2) => {
        const col1 = ctx.collections.find(x => x.id === card1.col);
        const col2 = ctx.collections.find(x => x.id === card2.col);
        return col1.promo && col2.promo && card1.col === card2.col;
    };

    return ctx.sendCfm(ctx, user, {
        question,
        force: ctx.globals.force,
        onConfirm: async (x) => {
            try {
                
                // Updated logic for filtering the result cards
                let res = ctx.cards.filter(x => 
                    x.level === card1.level && 
                    x.id != card1.id && 
                    x.id != card2.id && 
                    !isExcludedCollection(x) &&
                    (card1.tags === card2.tags ? x.tags === card1.tags : ['boygroups', 'girlgroups'].includes(x.tags))
                );
                
                // Apply collection-specific logic only if both cards are from the same promo collection
                if (isSameCollectionPromo(card1, card2)) {
                    res = res.filter(x => ctx.collections.find(y => y.id === x.col && y.promo));
                } else {
                    // Ensure that the resulting card is not from a promo collection
                    res = res.filter(x => !ctx.collections.find(y => y.id === x.col && y.promo));
                }

                
                const filterByCollection = (x) => {
                    const collection = ctx.collections.find(y => y.id === x.col);
                    return !collection.album && 
                        !collection.fragments && 
                        !collection.lottery && 
                        !collection.liveauction && 
                        !collection.jackpot && 
                        !collection.birthdays && 
                        !collection.limited;
                };
                
                const filterByTags = (x, condition) => {
                    return condition ? x.tags.includes(card1.tags[0]) : true;
                };                
                
                const filterBySameCollection = (x, condition) => {
                    return condition ? x.col === card1.col : true;
                };
                
                const colCondition = card1.col === card2.col;
                const tagsCondition = card1.tags === card2.tags;
                
                // res = res.filter(x => filterByCollection(x) && filterByTags(x, tagsCondition) && filterBySameCollection(x, colCondition) && !isExcludedCollection(x));
                res = res.filter(x => !isExcludedCollection(x) && filterByTags(x, tagsCondition) && filterBySameCollection(x, colCondition));

                
                const newcard = _.sample(res)
                user.vials += vialres
                user.exp -= cost

                let stats = await getStats(ctx, user, user.lastdaily)
                stats.forge += 1
                stats[`forge${card1.level}`] += 1
                stats.tomatoout += cost
                stats.vialin += vialres


                if(!newcard)
                    return ctx.reply(user, `an error occured, please try again`, 'red')

                await removeUserCards(ctx, user, [card1.id, card2.id])


                await addUserCards(ctx, user, [newcard.id])
                user.lastcard = newcard.id
                await completed(ctx, user, [card1.id, card2.id, newcard.id])
                await user.save()
                await saveAndCheck(ctx, user, stats)
                await evalCard(ctx, newcard)

                await plotPayout(ctx, 'smithhub', 1, 10)

                const usercards = await findUserCards(ctx, user, [newcard.id])
                    .select('amount')
                    
                return ctx.reply(user, {
                    image: { url: newcard.url },
                    color: colors.blue,
                    description: `you got ${formatName(newcard)}!
                        **${numFmt(vialres)}** ${ctx.symbols.vial} were added to your account
                        ${usercards[0].amount > 1 ? `*You already have this card*` : ''}`
                }, 'green', true)
            } catch(e) {
                return ctx.reply(user, `an error occured while executing this command. 
                    Please try again`, 'red', true)
            }
        }
    }, false)
})))


const asdate    = require('add-subtract-date')
const {firstBy} = require('thenby')

const {
    cap,
    tryGetUserID,
    nameSort,
    escapeRegex,
} = require('../utils/tools')

const { 
    bestColMatch, 
    bestColMatchMulti,
} = require('./collection')

const { 
    fetchTaggedCards,
} = require('./tag')

const { 
    evalCardFast,
} = require('./eval')

const { 
    fetchInfo,
} = require('./meta')

const {
    getUserCards,
} = require('./user')

const promoRarity = {
    halloween: '🎃',
    christmas: '❄',
    valentine: '🍫',
    birthdays: '🍰',
    halloween18: '🍬',
    christmas18: '🎄',
    valentine19: '💗',
    halloween19: '👻',
    christmas19: '☃️',
    birthday20: '🎈',
    limited: '🔥',
    special: '✨',
    twizonevent: '🌹',
    liveauction: '💎',
    halloween20: '🎃',
    izonesomeday: '🧩',
    loonaoec: '🧩',
    izonedaydream: '🧩',
    izonepinkblusher: '🧩',
    gugudansemina: '🧩',
    loonaonethird: '🧩',
    loonayyxy: '🧩',
    pristinv: '🧩',
    snsdohggsnsdtts: '🧩',
    wjmk: '🧩',
    wjsnchocome: '🧩',
    exocbx: '🧩',
    seventeenbss: '🧩',
    day6evenofday: '🧩',
    btobblue: '🧩',
    snsdohgg:'🧩',
    snsdtts: '🧩',
    orangecaramel: '🧩',
    btob4u: '🧩',
    lottery: '🎁',
    fanarts: '🎨',
    ninemusesa: '🧩',
    rainbowpixie: '🧩',
    rainbowblaxx: '🧩',
    superjuniorkry: '🧩',
    aoacream: '🧩',
    pinkfantasyshadow: '🧩',
    pinkfantasyshy: '🧩',
    spicas: '🧩',
    xmas20: '🎀',
    jackpot: '🎯',
    fanaticsflavor: '🧩',
    tripleh: '🧩',
    lunar: '🧧',
    wowthing: '🧩',
    berrygoodhh: '🧩',
    honeybee: '🧩',
    girlsnextdoor: '🧩',
    nuestw: '🧩',
    straybts: '🧬',
    onethestory: '🌸',
    blackvelvet :'🍨',
    ggalbums: '📀',
    bgalbums: '📀',
    easter21: '🐰',
    smileyevent: '☀️',
    hairintheair: '🧩',
    rgpbside: '🧩',
    wjsntheblack: '🧩',
    taran4: '🧩',
    elastu: '🧩',
    anniversary21: '🎉',
    sunnygirls: '🧩',
    teenteen: '🧩',
    purplehashtag: '🧩',
    got7teen: '🧭',
    itzidle: '💄',
    halloween21: '🔮',
    xmas21: '☃️',
    valentines22: '❣️',
    notfriends: '🧩',
    flowercrown: '🌺',
    monstaexo: '👾',
    summerevent: '🍹',
    signed: '💫',
    halloween23: '👻',
    winterevent23: '⛸️',
    mystical: '🦋',
    chuseok24: '🌕',
    petsevent: '🐾',
    xmas24: '🌧️'

}

const formatName = (x) => {
    const promo = promoRarity[x.col]
    const rarity = promo? `\`${new Array(x.level + 1).join(promo)}\`` : new Array(x.level + 1).join('✩')
    return `[${rarity}]${x.locked? ' `🔒`': ''}${x.fav? '<:ab_7heart:1169740480460890122>' : ''} [${cap(x.name.replace(/_/g, ' '))}](${x.shorturl}) \`[${x.col}]\``
}


const parseArgs = (ctx, user, option) => {
    const lastdaily = user? user.lastdaily: asdate.subtract(new Date(), 1, 'day')

    const cols = [], levels = [], keywords = []
    const anticols = [], antilevels = []
    let sort
    const q = { 
        ids: [], 
        sort: null,
        filters: [],
        tags: [],
        antitags: [],
        extra: [],
        sources: [],
        lastcard: false,
        diff: 0,
        fav: false,
        evalQuery: false,
        userQuery: false,
    }



    //Card Selection Related
    const cardOption = option || ctx.options.find(x => x.name === 'card_query' || x.name === `card_query_1` || x.name === 'card_query_2')
    const cardArgs = cardOption? cardOption.value.split(' ').map(x => x.toLowerCase()): []
    cardArgs.map(x => {
        let substr = x.substr(1)
        if(x === '.') {
            q.lastcard = true

        } else if((x[0] === '<' || x[0] === '>' || x[0] === '=' || x[0] === '\\') && x[1] != '@') {
            const lt = x[0] === '<'
            switch(substr) {
                case 'date':
                    sort = sortBuilder(ctx, sort,(a, b) => a.obtained - b.obtained, lt)
                    q.userQuery = true
                    break
                case 'amount':
                    sort = sortBuilder(ctx, sort,(a, b) => a.amount - b.amount, lt)
                    break
                case 'name':
                    sort = sortBuilder(ctx, sort,(a, b) => nameSort(a, b) , lt)
                    break
                case 'star':
                    sort = sortBuilder(ctx, sort,(a, b) => a.level - b.level , lt)
                    break
                case 'col':
                    sort = sortBuilder(ctx, sort,(a, b) => nameSort(a, b, "col") , lt)
                    break
                case 'eval':
                    sort = sortBuilder(ctx, sort,(a, b) => evalSort(ctx, a, b) , lt)
                    q.evalQuery = true
                    break
                case 'rating':
                    sort = sortBuilder(ctx, sort,(a, b) => (a.rating || 0) - (b.rating || 0), lt)
                    q.userQuery = true
                    break
                case 'levels':
                        sort = sortBuilder(ctx, sort,(a, b) => (a.exp || 0) - (b.exp || 0), lt)
                        q.userQuery = true
                    break
                    
                default: {
                    const eq = x[1] === '='
                    eq? substr = x.substr(2): substr
                    const escHeart = x[0] === '\\'
                    if (escHeart && x[1] === '<') {
                        x = x.substr(1)
                        substr = x.substr(1)
                    }
                    switch(x[0]) {
                        case '>' : q.filters.push(c => eq? c.amount >= substr: c.amount > substr); q.userQuery = true; break
                        case '<' : q.filters.push(c => eq? c.amount <= substr: c.amount < substr); q.userQuery = true; break
                        case '=' : q.filters.push(c => c.amount == substr); q.userQuery = true; break
                    }

                }
            }
        } else if(x[0] === '-' || x[0] === '!') {
            if(x[0] === '!' && x[1] === '#') {
                q.antitags.push(substr.substr(1))
            } else {
                const m = x[0] === '-'
                switch(substr) {
                    case 'gif': q.filters.push(c => c.animated == m); break
                    case 'multi': q.filters.push(c => m? c.amount > 1 : c.amount === 1); q.userQuery = true; break
                    case 'fav': q.filters.push(c => m? c.fav : !c.fav); m? q.fav = true: q.fav; q.userQuery = true; break
                    case 'lock':
                    case 'locked': q.filters.push(c => m? c.locked : !c.locked); m? q.locked = true: q.locked; q.userQuery = true; break
                    case 'new': q.filters.push(c => m? c.obtained > lastdaily : c.obtained <= lastdaily); q.userQuery = true; break
                    case 'rated': q.filters.push(c => m? c.rating: !c.rating); q.userQuery = true; break
                    case 'wish': q.filters.push(c => m? user.wishlist.includes(c.id): !user.wishlist.includes(c.id)); break
                    case 'promo': const mcol = bestColMatchMulti(ctx, substr); m? mcol.map(x=> cols.push(x.id)): mcol.map(x=> anticols.push(x.id)); break
                    case 'diff': q.diff = m? 1: 2; break
                    case 'miss': q.diff = m? 1: 2; break
                    case 'boygroups': q.filters.push(c => c.tags === 'boygroups'); break
                    case 'girlgroups': q.filters.push(c => c.tags === 'girlgroups'); break
                    case 'fragments': q.filters.push(c => c.fragments === true); break
                    default: {
                        const pcol = bestColMatch(ctx, substr)
                        if(m) {
                            if(parseInt(substr)) levels.push(parseInt(substr))
                            else if(pcol) cols.push(pcol.id)
                        } else {
                            if(parseInt(substr)) antilevels.push(parseInt(substr))
                            else if(pcol) anticols.push(pcol.id)
                        }
                    }
                }
            }
        } else if(x[0] === '#') {
            q.tags.push(substr.replace(/[^\w]/gi, ''))
        } else if(x[0] === ':') {
            q.extra.push(substr)
        } else if(x[0] === '$') {
            q.sources.push(substr)
        } else {
            const tryid = tryGetUserID(x)
            if(tryid) q.ids.push(tryid)
            else keywords.push(x)
        }
    })

    const userID = ctx.options.find(x => x.name === 'user_id')
    if(userID)
        q.ids.push(userID.value)

    //Tag Related
    const tagOption = ctx.options.find(x => x.name === 'tag')
    if(tagOption)
        q.tags.push(tagOption.value.replace(/[^\w]/gi, '_'))

    if(cols.length > 0) q.filters.push(c => cols.includes(c.col))
    if(levels.length > 0) q.filters.push(c => levels.includes(c.level))
    if(anticols.length > 0) q.filters.push(c => !anticols.includes(c.col))
    if(antilevels.length > 0) q.filters.push(c => !antilevels.includes(c.level))
    if(keywords.length > 0) 
        q.filters.push(c => (new RegExp(`(_|^)${keywords.map(k => escapeRegex(k)).join('.*')}`, 'gi')).test(c.name))

    q.isEmpty = (usetag = true) => {
        return !q.ids[0] && !q.lastcard && !q.filters[0] && !((q.tags[0] || q.antitags[0]) && usetag)
    }
    if (!sort)
        q.sort = firstBy((a, b) => b.level - a.level).thenBy("col").thenBy("name")
    else
        q.sort = sort

    return q
}

const evalSort = (ctx, a, b) => {
    if(evalCardFast(ctx, a) > evalCardFast(ctx, b))return 1
    if(evalCardFast(ctx, a) < evalCardFast(ctx, b))return -1
    return 0
}

const sortBuilder = (ctx, sort, sortby, lt) => {
    if (!sort)
        return firstBy(sortby, {direction: lt? "asc": "desc"})
    else
        return sort.thenBy(sortby, {direction: lt? "asc": "desc"})
}

const filter = (cards, query) => {
    query.filters.map(f => cards = cards.filter(f))
    //return cards.sort(nameSort)
    return cards
}

const equals = (card1, card2) => {
    return card1.name === card2.name && card1.level === card2.level && card1.col === card2.col
}

const addUserCard = (user, cardID) => {
    const matched = user.cards.findIndex(x => x.id == cardID)
    if(matched > -1) {
        user.cards[matched].amount++
        user.markModified('cards')
        return user.cards[matched].amount
    }

    user.cards.push({ id: cardID, amount: 1, obtained: new Date() })
    return 1
}

const removeUserCard = (ctx, user, cardID) => {
    const matched = user.cards.findIndex(x => x.id == cardID)
    const card = user.cards[matched]
    user.cards[matched].amount--
    user.cards = user.cards.filter(x => x.amount > 0)
    user.markModified('cards')

    if(card.amount === 0 && card.rating) {
        removeRating(ctx, cardID, card.rating)
    }

    return user.cards[matched]? user.cards[matched].amount : 0
}

const mapUserCards = (ctx, userCards) => {
    if (!Array.isArray(userCards) || !Array.isArray(ctx.cards)) {
        return []
    }

    // Create a Set of owned card IDs for faster lookup
    const ownedCardIds = new Set(userCards.map(uc => uc.cardid))

    return userCards
        .map(userCard => {
            // Ensure we have valid card data and user owns this card
            if (!userCard?.cardid || !ownedCardIds.has(userCard.cardid)) return null

            // Find matching card definition
            const card = ctx.cards.find(c => c.id === userCard.cardid)
            if (!card) return null

            // Double check ownership
            if (card.id !== userCard.cardid) return null

            // Ensure all required properties exist with defaults
            return {
                ...card,                    // Base card data
                amount: userCard.amount || 1,
                obtained: userCard.obtained || new Date(),
                fav: Boolean(userCard.fav),
                locked: Boolean(userCard.locked),
                exp: userCard.exp || 0,
                mark: userCard.mark || '',
                rating: userCard.rating || 0
            }
        })
        .filter(Boolean) // Remove any null entries
}

/**
 * Helper function to enrich the comamnd with user cards
 * @param  {Function} callback command handler
 * @return {Promise}
 */
const withCards = (callback) => async (ctx, user, args) => {
    if (!ctx?.cards || !user?.discord_id) {
        return ctx.reply(user, 'Error loading cards. Please try again.', 'red')
    }

    const userCards = await getUserCards(ctx, user)
    if (!userCards?.length) {
        return ctx.reply(user, `you don't have any cards. Get some using \`${ctx.prefix}claim cards\``, 'red')
    }

    // Create a Set of owned card IDs for validation
    const ownedCardIds = new Set(userCards.map(uc => uc.cardid))

    const map = mapUserCards(ctx, userCards)
    if (!map?.length) {
        return ctx.reply(user, 'Error mapping cards. Please try again.', 'red')
    }

    // Additional validation to ensure only owned cards are included
    let cards = filter(map, args)
    cards = cards.filter(card => ownedCardIds.has(card.id))

    // Apply filters safely
    if (args?.tags?.length > 0) {
        try {
            const tgcards = await fetchTaggedCards(args.tags)
            if (Array.isArray(tgcards)) {
                cards = cards.filter(x => tgcards.includes(x.id) && ownedCardIds.has(x.id))
            }
        } catch (err) {
            // Continue without tag filtering if it fails
        }
    }

    if (args?.antitags?.length > 0) {
        try {
            const tgcards = await fetchTaggedCards(args.antitags)
            if (Array.isArray(tgcards)) {
                cards = cards.filter(x => !tgcards.includes(x.id) && ownedCardIds.has(x.id))
            }
        } catch (err) {
            // Continue without antitag filtering if it fails
        }
    }

    if (args?.sources?.length > 0 && ctx.cardInfos) {
        try {
            const sourced = ctx.cardInfos
                .filter(x => x?.meta?.source && args.sources.includes(x.meta.source))
                .map(z => z.id)
            cards = cards.filter(x => sourced.includes(x.id) && ownedCardIds.has(x.id))
        } catch (err) {
            // Continue without source filtering if it fails
        }
    }

    if (args?.lastcard && user?.lastcard) {
        cards = map.filter(x => x.id === user.lastcard && ownedCardIds.has(x.id))
    }

    if (!cards?.length) {
        return ctx.reply(user, `no cards found matching \`${args.cardQuery || ''}\``, 'red')
    }

    // Safe sort with ownership validation
    if (typeof args?.sort === 'function') {
        try {
            cards = cards.filter(card => ownedCardIds.has(card.id))
            cards.sort(args.sort)
        } catch (err) {
            // Continue without sorting if it fails
        }
    }

    // Update lastcard safely
    if (!args?.lastcard && cards.length > 0) {
        try {
            const firstCard = cards[0]
            if (ownedCardIds.has(firstCard.id)) {
                user.lastcard = firstCard.id
                await user.save()
            }
        } catch (err) {
            // Continue without updating lastcard if it fails
        }
    }

    return callback(ctx, user, cards, args)
}

/**
 * Helper function to enrich the comamnd with selected card
 * @param  {Function} callback command handler
 * @return {Promise}
 */
const withGlobalCards = (callback) => async(ctx, user, args) => {
    let allcards
    if(args.userQuery){
        const userCards = await getUserCards(ctx, user)
        allcards = mapUserCards(ctx, userCards)
    } else {
        allcards = ctx.cards.slice()
    }

    let cards = filter(allcards, args)
    if(args.tags.length > 0) {
        const tgcards = await fetchTaggedCards(args.tags)
        cards = cards.filter(x => tgcards.includes(x.id))
    }

    if(args.antitags.length > 0) {
        const tgcards = await fetchTaggedCards(args.antitags)
        cards = cards.filter(x => !tgcards.includes(x.id))
    }

    if (args.sources.length > 0) {
        const sourced = ctx.cardInfos.filter(x => x.meta.source && args.sources.some(y => y === x.meta.source)).map(z => z.id)
        cards = cards.filter(x => sourced.includes(x.id))
    }

    if(args.lastcard)
        cards = [ctx.cards[user.lastcard]]

    if(cards.length == 0)
        return ctx.reply(user, `no cards found matching \`${args.cardQuery}\``, 'red')

    cards.sort(args.sort)

    if(!args.lastcard && cards.length > 0) {
        user.lastcard = cards[0].id
        await user.save()
    }

    return callback(ctx, user, cards, args)
}

/**
 * Helper function to enrich the comamnd with user cards
 * @param  {Function} callback command handler
 * @return {Promise}
 */
const withMultiQuery = (callback) => async (ctx, user, args) => {
    const parsedargs = [], cards = []
    parsedargs.push(args.cardArgs1)
    if (args.cardArgs2)
        parsedargs.push(args.cardArgs2)

    const userCards = await getUserCards(ctx, user)
    const map = mapUserCards(ctx, userCards)
    try {
        await Promise.all(parsedargs.map(async (x, i) => {
            if(x.lastcard)
                cards.push(map.filter(x => x.id === user.lastcard))
            else {
                let batch = filter(map, x)

                if(x.tags.length > 0) {
                    const tgcards = await fetchTaggedCards(x.tags)
                    batch = batch.filter(x => tgcards.includes(x.id))
                }

                if(x.antitags.length > 0) {
                    const tgcards = await fetchTaggedCards(x.antitags)
                    batch = batch.filter(x => !tgcards.includes(x.id))
                }

                batch.sort(x.sort)
                cards.push(batch)
            }

            if(cards[i].length == 0)
                throw new Error(`${i + 1}`)
        }))
    } catch (e) {
        return ctx.reply(user, `no cards found in request **#${e.message}**`, 'red')
    }

    return callback(ctx, user, cards, parsedargs, args)
}

const bestMatch = cards => cards? cards.sort((a, b) => a.name.length - b.name.length)[0] : undefined

const removeRating = async (ctx, id, rating) => {
    console.log(`removing rating ${id} ${rating}`)
    const info = fetchInfo(ctx, id)
    info.ratingsum -= rating
    info.usercount--
    await info.save()
}

module.exports = Object.assign(module.exports, {
    formatName,
    equals,
    bestMatch,
    addUserCard,
    removeUserCard,
    filter,
    parseArgs,
    withCards,
    withGlobalCards,
    mapUserCards,
    withMultiQuery,
    fetchInfo,
    removeRating,
})
