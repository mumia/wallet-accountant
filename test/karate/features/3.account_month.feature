Feature: Account month handling
  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }
    * def body =
    """
    {
      "bankName": "a bank",
      "name": "account month test",
      "accountType": "checking",
      "startingBalance": 100,
      "startingBalanceDate": "2023-08-01T00:00:00Z",
      "currency": "EUR"
    }
    """
    * def account = callonce read('creators/account_create.feature') body
    * def tag1 = callonce read('creators/tag_category_create.feature') {"category": "cat1", "tag": "tag1"}
    * def tag2 = callonce read('creators/tag_category_create.feature') {"category": "cat2", "tag": "tag2"}
    * def accountId = account.response.accountId
    * def tagId1 = tag1.response.tagId
    * def tagId2 = tag2.response.tagId

  Scenario: Account month operations
    # --------------------
    # Read account month initial state
    # --------------------
    Given path 'account-month', accountId
    When method get
    Then status 200
    And match response == read("accountMonth/response-read-account-month-no-movements.json")
    And response.accountId = accountId

    # --------------------
    # Register a debit
    # --------------------
    Given path 'account-month/account-movement'
    * def requestBody = read("accountMonth/request-first-register-new-movement-debit.json")
    * replace requestBody.${accountId} = accountId
    * replace requestBody.${tagId1} = tagId1
    * json requestJson = requestBody
    And request requestJson
    When method post
    Then status 201
    And match response == ""
    * sleep(1000)

    # --------------------
    # Read account month after debit
    # --------------------
    Given path 'account-month', accountId
    When method get
    Then status 200
    And match response == read("accountMonth/response-read-account-month-after-debit.json")
    And response.accountId = accountId
    And response.movements[0].tagIds[0] = tagId1

    # --------------------
    # Register a credit
    # --------------------
    Given path 'account-month/account-movement'
    * def requestBody = read("accountMonth/request-second-register-new-movement-credit.json")
    * replace requestBody.${accountId} = accountId
    * replace requestBody.${tagId2} = tagId2
    * json requestJson = requestBody
    And request requestJson
    When method post
    Then status 201
    And match response == ""
    * sleep(1000)

    # --------------------
    # Read account month after credit
    # --------------------
    Given path 'account-month', accountId
    When method get
    Then status 200
    And match response == read("accountMonth/response-read-account-month-after-credit.json")
    And response.accountId = accountId
    And response.movements[0].tagIds[0] = tagId1
    And response.movements[1].tagIds[0] = tagId2

    # --------------------
    # Register a credit
    # --------------------
    Given path 'account-month'
    * def requestBody = read("accountMonth/request-end-month.json")
    * replace requestBody.${accountId} = accountId
    * json requestJson = requestBody
    And request requestJson
    When method put
    Then status 204
    And match response == ""
    * sleep(1000)

    # --------------------
    # Retrieve account after month end
    # --------------------
    Given path 'account', accountId
    When method get
    Then status 200
    And match response.activeMonth ==
    """
      {"month":9, "year": 2023}
    """

    # --------------------
    # Read account month after new month
    # --------------------
    Given path 'account-month', accountId
    When method get
    Then status 200
    And match response == read("accountMonth/response-read-account-month-after-new-month.json")
    And response.accountId = accountId
