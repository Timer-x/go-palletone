*** Settings ***
Library           RequestsLibrary

*** Variables ***
${host}           http://localhost:8595/
# methods
${transerferPTNMethod}    wallet_transferPtn
${getBalanceMethod}    wallet_getBalance
${unlockAccountMethod}    personal_unlockAccount
${checkMediatorList}    mediator_listAll
${viewMediatorActives}    mediator_listActives
${voteMediator}    mediator_vote
${mediatorVoteResults}    mediator_listVoteResults
${personalListAccountsMethod}    personal_listAccounts
${IsActiveMediator}    mediator_isActive
# common variables
${userAccount}    ${null}
${userAccount2}    ${null}
${userAccount3}    ${null}
${userAccount4}    ${null}
${userAccount5}    ${null}
${tokenHolder}    ${null}
${mediatorHolder1}    ${null}
${mediatorHolder2}    ${null}
${mediatorHolder3}    ${null}
${mediatorHolder4}    ${null}
${mediatorHolder5}    ${null}
${mediatorActives1}    ${null}
${mediatorActives2}    ${null}
${mediatorActives3}    ${null}
${mediator1Result}    ${null}
${mediator2Result}    ${null}
${mediator3Result}    ${null}
${mediator4Result}    ${null}
${mediator5Result}    ${null}
${activeAccount1}    ${null}
${activeAccount2}    ${null}
${amount}         10000
${amount2}        20000
${amount3}        30000
${amount4}        40000
${amount5}        50000
${fee}            1
${pwd}            1
${duration}       600000000