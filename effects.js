const _ = require('lodash')
const { byAlias, completed } = require('../modules/collection')
const { formatName } = require('../modules/card')
const { addUserCards, getUserCards, findUserCards, getUserQuests} = require('../modules/user')
const { getStats } = require("../modules/userstats")
const { UserQuest } = require("../collections")
const asdate = require("add-subtract-date")
const { evalCard } = require("../modules/eval")

module.exports = [
    {
        id: 'tohrugift',
        name: 'Gift From Tohru',
        desc: 'Increase chances of getting a 3-star card every first claim per day',
        passive: true
    }, {
        id: 'cakeday',
        name: 'Cake Day',
        desc: 'Get +100 snowflakes in your daily for every claim you did',
        passive: true,
        animated: true
    }, {
        id: 'holygrail',
        name: 'The Holy Grail',
        desc: 'Get +25% of vials when liquifying 1 and 2-star cards',
        passive: true,
        animated: true
    }, {
        id: 'skyfriend',
        name: 'Skies Of Friendship',
        desc: 'Get 10% snowflakes back from wins on auction',
        passive: true
    }, {
        id: 'cherrybloss',
        name: 'Cherry Blossoms',
        desc: 'Any card forge is 50% cheaper',
        passive: true
    }, {
        id: 'onvictory',
        name: 'Onwards To Victory',
        desc: 'Get guild rank points 25% faster',
        passive: true
    }, {
        id: 'rulerjeanne',
        name: 'The Ruler Jeanne',
        desc: 'Get `/daily` every 17 hours instead of 20',
        passive: true
    }, {
        id: 'spellcard',
        name: 'Impossible Spell Card',
        desc: 'Usable effects have 40% less cooldown',
        passive: true
    }, {
        id: 'festivewish',
        name: 'Festival of Wishes',
        desc: 'Get notified when a card on your wishlist is auctioned',
        passive: true
    },
    {
        id: 'walpurgisnight',
        name: 'Walpurgis Night',
        desc: 'Draw few times per daily, maximum of 3 star per daily',
        passive: true
    },
    {
        id: 'enayano',
        name: 'Enlightened Ayano',
        desc: 'Completes tier 1 quest when used',
        passive: false,
        cooldown: 20,
        use: async (ctx, user) => {
            let quests = (await getUserQuests(ctx, user)).filter(x => x.type === 'daily' && !x.completed)
            const quest = ctx.quests.daily.find(y => quests?.some(z => z.questid === y.id) && y.tier === 1)
            if(!quest)
                return { msg: `you don't have any tier 1 quest to complete`, used: false }

            let stats = await getStats(ctx, user, user.lastdaily)
            quest.resolve(ctx, user, stats)
            quests = quests.filter(x => x.questid === quest.id)[0]
            await UserQuest.deleteOne(quests)
            stats.t1quests += 1
            await user.save()
            await stats.save()

            return { msg: `completed **${quest.name}**. You got ${quest.reward(ctx)}`, used: true }
        }
    }, {
        id: 'pbocchi',
        name: 'Powerful Bocchi',
        desc: 'Generates tier 1 quest when used',
        passive: false,
        cooldown: 32,
        use: async (ctx, user) => {
            const questList = (await getUserQuests(ctx, user)).filter(x => x.type === 'daily').map(x => x.questid)
            const quest = _.sample(ctx.quests.daily.filter(x => x.tier === 1 && !questList.includes(x.id) && x.can_drop))
            if(!quest)
                return { msg: `cannot find a unique quest. Please, complete some quests before using this effect.`, used: false }

            await UserQuest.create({userid: user.discord_id, questid: quest.id, type: 'daily', expiry: asdate.add(new Date(), 20, 'hours'), created: new Date()})

            return { msg: `received **${quest.name}**`, used: true }
        }
    }, {
        id: 'spaceunity',
        name: 'The Space Unity',
        desc: 'Gives random unique card from non-promo collection',
        passive: false,
        cooldown: 40,
        use: async (ctx, user, args) => {
            if (!args.extraArgs) {
                return { msg: "please specify collection in extra_arguments", used: false };
            }
            
            const name = args.extraArgs.replace(/^-/, "");
            const col = byAlias(ctx, name)[0];
            
            if (!col) {
                return { msg: `collection with ID '${args.extraArgs}' wasn't found`, used: false };
            }
            
            const restrictions = [
                { condition: col.promo, message: "cannot use this effect on promo collections" },
                { condition: col.fragments, message: "cannot use this effect on fragmented collections" },
                { condition: col.lottery, message: "cannot use this effect on lottery collections" },
                { condition: col.jackpot, message: "cannot use this effect on jackpot collections" },
                { condition: col.album, message: "cannot use this effect on album collections" },
            ];
            
            for (const restriction of restrictions) {
                if (restriction.condition) {
                    return { msg: restriction.message, used: false };
                }
            }
            
            const userCards = await getUserCards(ctx, user);
            const card = _.sample(ctx.cards.filter(x => x.col === col.id && !x.excluded && x.level < 4 && !userCards.some(y => y.cardid === x.id)));
            
            if (!card) {
                return { msg: `cannot fetch unique card from **${col.name}** collection`, used: false };
            }
            
            await addUserCards(ctx, user, [card.id]);
            user.lastcard = card.id;
            user.markModified("cards");
            await completed(ctx, user, [card.id]);
            await user.save();
            await evalCard(ctx, card)
            
            return { msg: `you got ${formatName(card)}`, img: card.url, used: true };
        }            
    }, {
        id: 'judgeday',
        name: 'The Judgment Day',
        desc: 'Grants effect of almost any usable card',
        passive: false,
        cooldown: 48,
        use: async (ctx, user, args) => {
            if(!args.extraArgs)
                return { msg: `please specify effect ID in extra_arguments`, used: false }

            const effectArgs = args.extraArgs.split(' ')
            const reg = new RegExp(effectArgs[0], 'gi')
            let effect = ctx.effects.filter(x => !x.passive).find(x => reg.test(x.id))

            if(!effect)
                return { msg: `effect with ID \`${effectArgs[0]}\` was not found or it is not usable`, used: false }

            let excludedEffects = ["memoryval", "memoryxmas", "memorybday", "memoryhall", "judgeday", "walpurgisnight"]

            if(excludedEffects.includes(effect.id))
                return { msg: `you cannot use that effect card with Judgment Day`, used: false }

            args.extraArgs = effectArgs.slice(1).join(' ')
            const res = await effect.use(ctx, user, args)
            return res
        }
    }, {
        id: 'claimrecall',
        name: 'Claim Recall',
        desc: 'Claim cost gets recalled by 4 claims, as if they never happened',
        passive: false,
        cooldown: 15,
        use: async (ctx, user) => {
            let stats = await getStats(ctx, user, user.lastdaily)

            if (stats.claims < 5)
                return { msg: `you can only use Claim Recall when you have claimed more than 4 cards!`, used: false }

            stats.claims -= 4
            await stats.save()
            return { msg: `claim cost has been reset to **${stats.claims * 50}**`, used: true }
        }
    }
]
