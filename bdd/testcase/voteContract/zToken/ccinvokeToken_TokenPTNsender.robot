*** Settings ***
Suite Setup       voteTransToken
Default Tags      normal
Library           ../../utilFunc/createToken.py
Resource          ../../utilKwd/utilVariables.txt
Resource          ../../utilKwd/normalKwd.txt
Resource          ../../utilKwd/utilDefined.txt
Resource          ../../utilKwd/behaveKwd.txt

*** Variables ***
${votePTN}        1000

*** Test Cases ***
Scenario: Vote - Ccinvoke Token
    [Documentation]    Verify Sender's PTN and VOTE value
    #${geneAdd}    Given Get genesis address
    ${PTN1}    ${item1}    ${voteToken}    And Request getbalance before create token
    ${resp}    When Ccinvoke token of vote contract    ${voteToken}
    ${PTN'}    ${item'}    And Calculate gain of recieverAdd    ${PTN1}    ${item1}
    ${PTN2}    ${item2}    And Request getbalance after create token    ${voteToken}
    Then Assert gain of reciever    ${PTN'}    ${PTN2}    ${item'}    ${item2}

*** Keywords ***
Get genesis address
    ${geneAdd}    getGeneAdd    ${host}
    [Return]    ${geneAdd}

Request getbalance before create token
    #    [Arguments]    ${geneAdd}    ${voteToken}
    ${PTN1}    ${result1}    normalGetBalance    ${listAccounts[0]}
    sleep    4
    ${voteToken}    getTokenId    ${voteId}    ${result1['result']}
    ${item1}    Get From Dictionary    ${result1['result']}    ${voteToken}
    [Return]    ${PTN1}    ${item1}    ${voteToken}

Ccinvoke token of vote contract
    [Arguments]    ${voteToken}
    ${supportList}    Create List    support    ${supportSection}
    ${ccList}    Create List    ${listAccounts[0]}    ${destructionAdd}    ${voteToken}    ${votePTN}    ${PTNPoundage}
    ...    ${voteContractId}    ${supportList}    ${pwd}    ${duration}
    ${resp}    setPostRequest    ${host}    ${invokeTokenMethod}    ${ccList}
    [Return]    ${resp}

Calculate gain of recieverAdd
    [Arguments]    ${PTN1}    ${item1}
    ${item'}    Evaluate    ${item1}-${votePTN}
    #${totalGain}    Evaluate    int(${votePTN})-int(${PTNPoundage})
    #${GAIN}    countRecieverPTN    ${totalGain}
    ${PTN'}    Evaluate    ${PTN1}-${PTNPoundage}
    [Return]    ${PTN'}    ${item'}

Request getbalance after create token
    [Arguments]    ${voteToken}
    sleep    4
    ${PTN2}    ${result2}    normalGetBalance    ${listAccounts[0]}
    ${item2}    Get From Dictionary    ${result2['result']}    ${voteToken}
    [Return]    ${PTN2}    ${item2}

Assert gain of reciever
    [Arguments]    ${PTN'}    ${PTN2}    ${item'}    ${item2}
    Should Be Equal As Strings    ${item2}    ${item'}
    Should Be Equal As Numbers    ${PTN2}    ${PTN'}
