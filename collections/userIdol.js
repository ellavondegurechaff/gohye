const {model, Schema} = require ('mongoose')

module.exports = model ('UserIdol', {

    ownerid:            { type: String, index: true },
    idolid:             { type: Number, index: true },
    idolname:           { type: String, default: '' },
    idolgroup:          { type: String, default: '' },
    
    
    idolstats: {
        rating:         { type: Number, default: 0 },
        level:          { type: Number, default: 0 },
        experience:     { type: Number, default: 0 },
        starlevel:      { type: Number, default: 0 },
        bondpoints:     { type: Number, default: 0 },
    },

    skills: {
        singing:        { type: Number, default: 0 },
        dancing:        { type: Number, default: 0 },
        leadership:     { type: Number, default: 0 },
        visuals:        { type: Number, default: 0 },
        variety:        { type: Number, default: 0 },
        rapping:        { type: Number, default: 0 },
    }



})