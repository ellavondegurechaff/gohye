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