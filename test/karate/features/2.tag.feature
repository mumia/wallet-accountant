Feature: Tag handling

  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }

  Scenario: All tags read returns an empty list
    Given path "/tags"
    When method get
    Then status 200
    And match response == []

  Scenario: Tag category create and retrieve
    # --------------------
    # Create tag category "locations" with tag "Vieira"
    # --------------------
    Given path "/tag-category"
    And request read("tag/request-first-add-new-category-locations-tag-vieira.json")
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-new-category.json")
    * def tagCategoryId1 = response.tagCategoryId
    * def tagId1 = response.tagId
    * sleep(1000)

    # --------------------
    # Retrieving all tags returns only category "locations" with tag "Vieira"
    # --------------------
    Given path "/tags"
    When method get
    Then status 200
    And match response == read("tag/response-read-tags-only-first.json")
    And match response[0].tagCategoryId == tagCategoryId1
    And match response[0].tags[0].tagId == tagId1

    # --------------------
    # Create tag category "insurance" with tag "Car"
    # --------------------
    Given path "/tag-category"
    And request read("tag/request-second-add-new-category-insurance-tag-car.json")
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-new-category.json")
    * def tagCategoryId2 = response.tagCategoryId
    * def tagId2 = response.tagId
    * sleep(1000)

    # --------------------
    # Retrieving all tags returns both categories with their single tags
    # --------------------
    Given path "/tags"
    When method get
    Then status 200
    And match response == read("tag/response-read-tags-first-and-second.json")
    And match response[0].tagCategoryId == tagCategoryId1
    And match response[0].tags[0].tagId == tagId1
    And match response[1].tagCategoryId == tagCategoryId2
    And match response[1].tags[0].tagId == tagId2

    # --------------------
    # Add tag "Berlin" to category "locations"
    # --------------------
    Given path "/tag"
    * def requestBody = read("tag/request-third-add-berlin-tag-to-locations-category.json")
    * replace requestBody.${tagCategoryId1} = tagCategoryId1
    * json requestJson = requestBody
    And request requestJson
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-existing-category.json")
    * def tagCategoryId3 = response.tagCategoryId
    * def tagId3 = response.tagId

    # --------------------
    # Add tag "House" to category "insurance"
    # --------------------
    Given path "/tag"
    * def requestBody = read("tag/request-fourth-add-house-tag-to-insurance-category.json")
    * replace requestBody.${tagCategoryId2} = tagCategoryId2
    * json requestJson = requestBody
    And request requestJson
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-existing-category.json")
    * def tagId4 = response.tagId
    * sleep(1000)

    # --------------------
    # Retrieving all tags returns both categories with their dual tags
    # --------------------
    Given path "/tags"
    When method get
    Then status 200
    And match response == read("tag/response-read-tags-first-second-third-and-fourth.json")
    And match response[0].tagCategoryId == tagCategoryId1
    And match response[0].tags[0].tagId == tagId1
    And match response[0].tags[1].tagId == tagId3
    And match response[1].tagCategoryId == tagCategoryId2
    And match response[1].tags[0].tagId == tagId2
    And match response[1].tags[1].tagId == tagId4