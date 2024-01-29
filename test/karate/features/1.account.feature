Feature: Account handling
  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }
#    * def account = callonce read('creators/account.feature')

  Scenario: All accounts read returns an empty list
    Given path "accounts"
    When method get
    Then status 200
    And match response == []

  Scenario: Account registration and retrieval
    # --------------------
    # Add first account
    # --------------------
    Given path "account"
    And request read("account/request-first-register-account.json")
    When method post
    Then status 201
    And match response == read("account/response-register-account.json")
    * def accountId1 = response.accountId
    * sleep(1000)

    # --------------------
    # Retrieve first account
    # --------------------
    Given path 'account', accountId1
    When method get
    Then status 200
    And match response == read("account/response-read-first-account.json")

    # --------------------
    # Retrieve all accounts returns first account only
    # --------------------
    Given path "accounts"
    When method get
    Then status 200
    And match response == read("account/response-read-all-accounts-only-first.json")
    And match response[0].accountId == accountId1

    # --------------------
    # Add second account
    # --------------------
    Given path "account"
    And request read("account/request-second-register-account.json")
    When method post
    Then status 201
    And match response == read("account/response-register-account.json")
    * def accountId2 = response.accountId
    * sleep(1000)

    # --------------------
    # Retrieve second account
    # --------------------
    Given path 'account', accountId2
    When method get
    Then status 200
    And match response == read("account/response-read-second-account.json")

    # --------------------
    # Retrieve all accounts returns both first and second accounts
    # --------------------
    Given path "accounts"
    When method get
    Then status 200
    And match response == read("account/response-read-all-accounts-first-and-second.json")
    And match response[0].accountId == accountId1
    And match response[1].accountId == accountId2
