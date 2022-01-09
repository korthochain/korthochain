http Ip: http://152.32.189.122:13789/
request type: POST
json body:
    //get nonce,usage：
    {"method":"nonce","address":"0xd7863649D0A766D0022167b1fd4cfBF0743aA95"}

    //get kto balance,usage：
    {"method":"balance","address":"0xd7863649D0A766D0022167b1fd4cfBF0743aA95"}

    //get transaction by hash,usage：
    {"method":"transaction","hash":"7468c23c66fd1a3b92dcb88db39f2e4154ebc2965c99f1314283e31f07631e38"}

    //get the latest block,usage：
    {"method":"block","option":"latest"}

    //get block by hash,usage：
    {"method":"block","option":"hash","hash":"efea83b34aaed42b29379c3396b0ef5c2f2b09733c3561174e71dfe2950bbb0c"}

    //get block by height,usage：
    {"method":"block","option":"height","height":"88200"}

    //get token-10 decimal,usage：
    {"method":"token_10","option":"decimal","symbol":"CM"}

    //get token-10 balance,usage：
   {"method":"token_10","option":"balance","symbol":"GPS","address":"Kto46nMQJTbLJ3hHAwgUXmBXaq7VB65R37LFj4d5zr4aPPm"}

    //get get Contract name/symbol/decimal/totalSupply,usage 
    {"method":"token_20","option":"data","contract":"0x12584E92430cb23b246CAF46bF81b2Dd946883e9"}

    //get token-20 balance,usage:
    {"method":"token_20","option":"balance","contract":"0x12584E92430cb23b246CAF46bF81b2Dd946883e9","address":"0x5C93F5Ba8CCe6626213b9b9Ae8C4da26e77F76A6"}

    //get total pledge or total mined, usage: 
    {"method":"pledge","address":"otKDjCTT7fyrmraBwSV3pFkfY99rPziBWFBSSokHgjUQpGx"}

        