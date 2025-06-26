const dateFormat    = require('dateformat')
const msToTime      = require('pretty-ms')
const _             = require('lodash')
const {firstBy}     = require("thenby")


const {
    byAlias, 
    reset,
    resetNeeds,
    hasResetNeeds,
} = require('../modules/collection')

const {
    formatName,
    mapUserCards,
} = require('../modules/card')

const {
    nameSort,
    numFmt,
} = require('../utils/tools')

const {
    fetchOnly,
    findUserCards,
    getUserCards,
} = require('../modules/user')

const {
    withInteraction,
} = require("../modules/interactions")

const {cmd}         = require('../utils/cmd')
const colors        = require('../utils/colors')
const {calculateCollectionEval} = require('../modules/eval')
const UserCard      = require('../collections/userCard')
const Users         = require('../collections/user')
const AsciiTable = require('ascii-table');


cmd(['collection', 'list'], withInteraction(async (ctx, user, args) => {
    let cols
    if (args.cols.length > 0)
        cols = _.flattenDeep(args.cols)
    else
        cols = byAlias(ctx, ``)

    const completed = args.completed? true: args.completed === false
    const clouted = args.clouted? true: args.clouted === false
    const sort = args.sortComplete? true: args.sortComplete === false

    const userCards = await getUserCards(ctx, user)

    cols = _.uniqBy(cols, 'id').sort((a, b) => nameSort(a, b, 'id')).filter(x => x)

    if(completed) {
        cols = cols.filter(x => {
            const isFragmentCollection = ctx.collections.filter(c => c.id === x.id && c.fragments).length > 0
            const cardsInCollection = ctx.cards.filter(c => c.col === x.id)
    
            if(isFragmentCollection) {
                // consider only 1-star cards for fragment collections
                const oneStarCards = cardsInCollection.filter(c => c.level === 1)
                return oneStarCards.every(c => userCards.some(uc => uc.cardid === c.id))
            } else {
                // consider all cards for non-fragment collections
                return cardsInCollection.every(c => userCards.some(uc => uc.cardid === c.id))
            }
        })
    }
    

    if (clouted) {
        if(args.clouted)
            cols = cols.filter(x => user.cloutedcols.some(y => y.id === x.id))
        else
            cols = cols.filter(x => !user.cloutedcols.some(y => y.id === x.id))
    }

    if(cols.length === 0)
        return ctx.reply(user, `no collections found`, 'red')

        const colList = cols.map(x => {
            const isFragmentCollection = ctx.collections.filter(c => c.id === x.id && c.fragments).length > 0
            const cardsInCollection = ctx.cards.filter(c => c.col === x.id)
            const clout = user.cloutedcols? user.cloutedcols.find(y => x.id === y.id): null
        
            let overall, usercount, rate
        
            if(isFragmentCollection) {
                // consider only 1-star cards for fragment collections
                const oneStarCards = cardsInCollection.filter(c => c.level === 1)
                overall = oneStarCards.length
                usercount = userCards.filter(c => oneStarCards.find(card => card.id === c.cardid)).length
            } else {
                // consider all cards for non-fragment collections
                overall = cardsInCollection.length
                usercount = userCards.filter(c => cardsInCollection.find(card => card.id === c.cardid)).length
            }
        
            rate = Math.ceil((usercount / overall) * 100)
            const cloutCount = clout ? clout.amount : 0
        
            return {
                colName: x.name,
                colID: x.id,
                clouted: cloutCount,
                allCards: overall,
                owned: usercount,
                perc: rate
            }
        })
        

    if (sort && !args.completed) {
        if (args.sortComplete)
            colList.sort(firstBy((a, b) => b.perc - a.perc).thenBy((c, d) => d.owned - c.owned).thenBy((e, f) => e.colName - f.colName))
        else
            colList.sort(firstBy((a, b) => a.perc - b.perc).thenBy((c, d) => c.owned - d.owned).thenBy((e, f) => e.colName - f.colName))
    }

    const pages = ctx.pgn.getPages(colList.map(x => {
        const cloutStars = x.clouted > 0? `[${x.clouted}${ctx.symbols.star}] `: ''
        const percText = x.perc > 0? x.perc < 1? '(<1%)': `(${x.perc}%)`: ''
        const countText = x.perc >= 100? '': `[${x.owned}/${x.allCards}]`
        return `${cloutStars}**${x.colName}** \`${x.colID}\` ${percText} ${countText}`
    }))

    return ctx.sendPgn(ctx, user, {
        pages,
        buttons: ['back', 'forward'],
        embed: {
            author: { name: `found ${cols.length} collections` }
        }
    })
})).access('dm')

cmd(['collection', 'info'], withInteraction(async (ctx, user, args) => {
    const col = _.flattenDeep(args.cols)[0];

    if(!col)
        return ctx.reply(user, `found 0 collections matching \`${args.colQuery}\``, 'red')

    const colCards = ctx.cards.filter(x => x.col === col.id && x.level < 5)
    const userCards = await findUserCards(ctx, user, colCards.map(x => x.id))
    const card = _.sample(colCards)
    const clout = user.cloutedcols.find(x => x.id === col.id)
    const colInfos = colCards.map(x => ctx.cardInfos[x.id]).filter(x => x)
    const ratingSum = colInfos.reduce((acc, cur) => acc + cur.ratingsum, 0)
    const ratingAvg = ratingSum / colInfos.reduce((acc, cur) => acc + cur.usercount, 0)

    const resp = []
    resp.push(`Overall cards: **${numFmt(colCards.length)}**`)
    resp.push(`You have: **${numFmt(userCards.length)} (${((userCards.length / colCards.length) * 100).toFixed(2)}%)**`)
    resp.push(`Average rating: **${ratingAvg.toFixed(2)}**`)

    const date = new Date(col.dateAdded)
    // resp.push(`Added: **${dateFormat(date, "yyyy-mm-dd")}** (${msToTime(new Date() - date, {compact: true})})`)

    if(col.author) {
        var author = await fetchOnly(col.author)

        if (author) {
            resp.push(`Template author: **${author.username}**`)
        }
    }

    if(clout && clout.amount > 0)
        resp.push(`Your clout: **${new Array(clout.amount + 1).join('★')}** (${clout.amount})`)

    resp.push(`Aliases: **${col.aliases.join(" **|** ")}**`)

    if(col.origin) 
        resp.push(`[More information about fandom](${col.origin})`)

    resp.push(`Sample card: ${formatName(card)}`)

    return ctx.send(ctx.interaction, {
        title: col.name,
        image: { url: card.url },
        description: resp.join('\n'),
        color: colors.blue
    }, user.discord_id)
})).access('dm')

cmd(['collection', 'reset'], withInteraction(async (ctx, user, args) => {
    const col = _.flattenDeep(args.cols)[0];

    if(!col)
        return ctx.reply(user, `found 0 collections matching \`${args.colQuery}\``, 'red')

    const isFragmentCollection = ctx.collections.filter(c => c.id === col.id && c.fragments).length > 0;
    let legendary, colCards, neededForReset, hasNeeded;

    if(isFragmentCollection){
        // Fragment Collection Reset Process
        legendary = ctx.cards.find(x => x.col === col.id && x.level === 4)
        colCards = ctx.cards.filter(x => x.col === col.id && x.level === 1)  // only consider 1-star cards
        neededForReset = await resetNeeds(ctx, user, colCards)
    } else {
        // Normal Collection Reset Process
        legendary = ctx.cards.find(x => x.col === col.id && x.level === 5)
        colCards = ctx.cards.filter(x => x.col === col.id && x.level < 5)
        neededForReset = await resetNeeds(ctx, user, colCards)
    }

    const matchingUserCards = await findUserCards(ctx, user, colCards.map(x => x.id))
    const userCards = mapUserCards(ctx, matchingUserCards)
    hasNeeded = await hasResetNeeds(ctx, userCards, neededForReset)

    let neededBlock = "";
    let percentageBlock = "";
    
    if (neededForReset[4] > 0) {
        const userCardsCount = userCards.filter((x) => x.level === 4 && x.amount > 0).length;
        // Check if user already has the 4-star fragment and this is a fragment collection
        const has4StarFragment = isFragmentCollection && userCards.some((x) => x.level === 4 && x.amount > 0);
        
        if (!has4StarFragment) {
            const percentage = Math.min(100, Math.round((userCardsCount / neededForReset[4]) * 100));
            neededBlock += `★★★★: **${neededForReset[4]}** - You have **${userCardsCount}** (${percentage}%) \n`;
            percentageBlock += `★★★★: ${percentage}%\n`;
        }
    }
    
    
    if (neededForReset[3] > 0) {
        const userCardsCount = userCards.filter((x) => x.level === 3 && x.amount > 0).length;
        const percentage = Math.min(100, Math.round((userCardsCount / neededForReset[3]) * 100));
        neededBlock += `★★★: **${neededForReset[3]}** - You have **${userCardsCount}** (${percentage}%) \n`;
        percentageBlock += `★★★: ${percentage}%\n`;
    }
    
    if (neededForReset[2] > 0) {
        const userCardsCount = userCards.filter((x) => x.level === 2 && x.amount > 0).length;
        const percentage = Math.min(100, Math.round((userCardsCount / neededForReset[2]) * 100));
        neededBlock += `★★: **${neededForReset[2]}** - You have **${userCardsCount}** (${percentage}%) \n`;
        percentageBlock += `★★: ${percentage}%\n`;
    }
    
    if (neededForReset[1] > 0) {
        const userCardsCount = userCards.filter((x) => x.level === 1 && x.amount > 0).length;
        const percentage = Math.min(100, Math.round((userCardsCount / neededForReset[1]) * 100));
        neededBlock += `★: **${neededForReset[1]}** - You have **${userCardsCount}** (${percentage}%) \n`;
        percentageBlock += `★: ${percentage}%\n`;
    }

    // Calculate the total evaluation price of the collection
    const totalEvalPrice = await calculateCollectionEval(ctx, col.id);

    // Determine the percentage to return (example: 30%)
    const percentageToReturn = 0.30;

    // Calculate the flakes to return
    const flakesToReturn = Math.floor(totalEvalPrice * percentageToReturn);
    
    
    if (!hasNeeded) {
        return ctx.reply(
            user,
            `To reset the collection **${col.name}** (\`${col.id}\`), you need to have **100%** of the required card rarities. The favorited cards are also included in percentage, You can reset the collection by unfavoriting the cards you've collected in this specific collection.\nUnique cards needed for this collection reset are as follows:\n${neededBlock}`,
            "red"
        );
    }

    if(isFragmentCollection){
        // Fragment Collection Reset Confirmation Question
        question = `Do you really want to reset **${col.name}**?
            This will take at random the following card rarities and amounts:
            ${neededBlock}
            You will lose all 1 copy of each card from that fragmented collection, gain 1 clout star and you'll get a **${legendary.name.toUpperCase()}** card.`
    } else {
        // Normal Collection Reset Confirmation Question
        question = `Do you really want to reset **${col.name}**?
            This will take at random the following card rarities and amounts:
            ${neededBlock} \n
            *Flakeback Effect* - You'll get **${flakesToReturn}\`❄️\`** 30% of the total eval of the whole collection which is **${totalEvalPrice}\`❄️\`** \n
            You will get a clout star ${legendary? ' + legendary ticket for resetting this collection' :
            `for resetting this collection\n> Please note that you won't get a legendary card ticket because this collection doesn't have any legendaries`}`
    }
    
    return ctx.sendCfm(ctx, user, {
        question,
        onConfirm: (x) => reset(ctx, user, col, neededForReset),
    })
}))

cmd(['collection', 'progress'], withInteraction(async (ctx, user, args) => {
    const collectionName = args.colQuery;

    // Fetch the collection
    const collection = ctx.collections.find(col => col.name.toLowerCase() === collectionName.toLowerCase());
    if (!collection) {
        return ctx.reply(user, `Collection "${collectionName}" not found.`, 'red');
    }

    // Fetch all cards in the collection, excluding legendary (level 5)
    const colCards = ctx.cards.filter(card => card.col === collection.id && card.level < 5);
    const colCardIds = colCards.map(card => card.id);

    // Calculate progress for each user directly in the database
    const userProgress = await UserCard.aggregate([
        { $match: { cardid: { $in: colCardIds } } },
        { $group: { _id: "$userid", count: { $sum: 1 } } },
        { $project: { discord_id: "$_id", progress: { $multiply: [ { $divide: ["$count", colCards.length] }, 100 ] } } }
    ]);

    // Fetch usernames in parallel
    const users = await Users.find({ discord_id: { $in: userProgress.map(u => u.discord_id) } });
    const usernameMap = users.reduce((acc, user) => {
        acc[user.discord_id] = user.username;
        return acc;
    }, {});

    // Attach usernames to progress and sort
    const progressWithUsernames = userProgress
        .map(u => ({ username: usernameMap[u.discord_id], progress: u.progress }))
        .sort((a, b) => b.progress - a.progress);

    // Create ASCII table
    let table = new AsciiTable('Collection Progress');
    table.setHeading('Username', 'Completion %');

    progressWithUsernames.slice(0, 10).forEach((u, index) => {
        table.addRow(u.username, `${u.progress.toFixed(2)}%`);
    });

    // Display results
    let embed = {
        color: colors.blue,
        description: `\`\`\`${table.toString()}\`\`\``
    };
    
    return ctx.interaction.createFollowup({embeds: [embed]});
})).access('dm');