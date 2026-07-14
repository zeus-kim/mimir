package fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/user/mimir-mcp/internal/db"
)

// FoodFetcher handles fetching from various food/nutrition APIs
type FoodFetcher struct {
	USDABaseURL        string
	OpenFoodFactsURL   string
	TheMealDBURL       string
	SpoonacularURL     string
	USDAAPIKey         string
	SpoonacularAPIKey  string
	client             *http.Client
}

// NewFoodFetcher creates a new FoodFetcher with API endpoints configured
func NewFoodFetcher() *FoodFetcher {
	return &FoodFetcher{
		USDABaseURL:        "https://api.nal.usda.gov/fdc/v1",
		OpenFoodFactsURL:   "https://world.openfoodfacts.org/api/v0",
		TheMealDBURL:       "https://www.themealdb.com/api/json/v1/1",
		SpoonacularURL:     "https://api.spoonacular.com",
		USDAAPIKey:         os.Getenv("USDA_API_KEY"),
		SpoonacularAPIKey:  os.Getenv("SPOONACULAR_API_KEY"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============================================================================
// USDA FoodData Central Types
// ============================================================================

type usdaSearchResponse struct {
	TotalHits int        `json:"totalHits"`
	Foods     []usdaFood `json:"foods"`
}

type usdaFood struct {
	FdcId           int           `json:"fdcId"`
	Description     string        `json:"description"`
	DataType        string        `json:"dataType"`
	BrandOwner      string        `json:"brandOwner,omitempty"`
	BrandName       string        `json:"brandName,omitempty"`
	Ingredients     string        `json:"ingredients,omitempty"`
	ServingSize     float64       `json:"servingSize,omitempty"`
	ServingSizeUnit string        `json:"servingSizeUnit,omitempty"`
	FoodNutrients   []usdaNutrient `json:"foodNutrients"`
}

type usdaNutrient struct {
	NutrientId    int     `json:"nutrientId"`
	NutrientName  string  `json:"nutrientName"`
	NutrientNumber string `json:"nutrientNumber"`
	UnitName      string  `json:"unitName"`
	Value         float64 `json:"value"`
}

// ============================================================================
// Open Food Facts Types
// ============================================================================

type openFoodSearchResponse struct {
	Count    int               `json:"count"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Products []openFoodProduct `json:"products"`
}

type openFoodProduct struct {
	Code              string            `json:"code"`
	ProductName       string            `json:"product_name"`
	Brands            string            `json:"brands"`
	Categories        string            `json:"categories"`
	IngredientsText   string            `json:"ingredients_text"`
	NutritionGrades   string            `json:"nutrition_grades"`
	ImageURL          string            `json:"image_url"`
	Nutriments        openFoodNutriments `json:"nutriments"`
	ServingSize       string            `json:"serving_size"`
	Quantity          string            `json:"quantity"`
	Countries         string            `json:"countries"`
	NutriScoreScore   int               `json:"nutriscore_score"`
}

type openFoodNutriments struct {
	EnergyKcal100g      float64 `json:"energy-kcal_100g"`
	Fat100g             float64 `json:"fat_100g"`
	SaturatedFat100g    float64 `json:"saturated-fat_100g"`
	Carbohydrates100g   float64 `json:"carbohydrates_100g"`
	Sugars100g          float64 `json:"sugars_100g"`
	Fiber100g           float64 `json:"fiber_100g"`
	Proteins100g        float64 `json:"proteins_100g"`
	Salt100g            float64 `json:"salt_100g"`
	Sodium100g          float64 `json:"sodium_100g"`
}

// ============================================================================
// TheMealDB Types
// ============================================================================

type mealDBSearchResponse struct {
	Meals []mealDBMeal `json:"meals"`
}

type mealDBMeal struct {
	IdMeal          string `json:"idMeal"`
	StrMeal         string `json:"strMeal"`
	StrCategory     string `json:"strCategory"`
	StrArea         string `json:"strArea"`
	StrInstructions string `json:"strInstructions"`
	StrMealThumb    string `json:"strMealThumb"`
	StrTags         string `json:"strTags"`
	StrYoutube      string `json:"strYoutube"`
	StrSource       string `json:"strSource"`
	// Ingredients and measures - MealDB uses numbered fields
	StrIngredient1  string `json:"strIngredient1"`
	StrIngredient2  string `json:"strIngredient2"`
	StrIngredient3  string `json:"strIngredient3"`
	StrIngredient4  string `json:"strIngredient4"`
	StrIngredient5  string `json:"strIngredient5"`
	StrIngredient6  string `json:"strIngredient6"`
	StrIngredient7  string `json:"strIngredient7"`
	StrIngredient8  string `json:"strIngredient8"`
	StrIngredient9  string `json:"strIngredient9"`
	StrIngredient10 string `json:"strIngredient10"`
	StrIngredient11 string `json:"strIngredient11"`
	StrIngredient12 string `json:"strIngredient12"`
	StrIngredient13 string `json:"strIngredient13"`
	StrIngredient14 string `json:"strIngredient14"`
	StrIngredient15 string `json:"strIngredient15"`
	StrMeasure1     string `json:"strMeasure1"`
	StrMeasure2     string `json:"strMeasure2"`
	StrMeasure3     string `json:"strMeasure3"`
	StrMeasure4     string `json:"strMeasure4"`
	StrMeasure5     string `json:"strMeasure5"`
	StrMeasure6     string `json:"strMeasure6"`
	StrMeasure7     string `json:"strMeasure7"`
	StrMeasure8     string `json:"strMeasure8"`
	StrMeasure9     string `json:"strMeasure9"`
	StrMeasure10    string `json:"strMeasure10"`
	StrMeasure11    string `json:"strMeasure11"`
	StrMeasure12    string `json:"strMeasure12"`
	StrMeasure13    string `json:"strMeasure13"`
	StrMeasure14    string `json:"strMeasure14"`
	StrMeasure15    string `json:"strMeasure15"`
}

// GetIngredients extracts all non-empty ingredients with their measures
func (m *mealDBMeal) GetIngredients() string {
	ingredients := []string{}
	pairs := []struct{ ing, meas string }{
		{m.StrIngredient1, m.StrMeasure1},
		{m.StrIngredient2, m.StrMeasure2},
		{m.StrIngredient3, m.StrMeasure3},
		{m.StrIngredient4, m.StrMeasure4},
		{m.StrIngredient5, m.StrMeasure5},
		{m.StrIngredient6, m.StrMeasure6},
		{m.StrIngredient7, m.StrMeasure7},
		{m.StrIngredient8, m.StrMeasure8},
		{m.StrIngredient9, m.StrMeasure9},
		{m.StrIngredient10, m.StrMeasure10},
		{m.StrIngredient11, m.StrMeasure11},
		{m.StrIngredient12, m.StrMeasure12},
		{m.StrIngredient13, m.StrMeasure13},
		{m.StrIngredient14, m.StrMeasure14},
		{m.StrIngredient15, m.StrMeasure15},
	}

	for _, p := range pairs {
		ing := strings.TrimSpace(p.ing)
		meas := strings.TrimSpace(p.meas)
		if ing != "" {
			if meas != "" {
				ingredients = append(ingredients, meas+" "+ing)
			} else {
				ingredients = append(ingredients, ing)
			}
		}
	}
	return strings.Join(ingredients, "; ")
}

// ============================================================================
// Spoonacular Types (optional, requires API key)
// ============================================================================

type spoonacularSearchResponse struct {
	Results      []spoonacularRecipe `json:"results"`
	TotalResults int                 `json:"totalResults"`
}

type spoonacularRecipe struct {
	Id               int      `json:"id"`
	Title            string   `json:"title"`
	Image            string   `json:"image"`
	ImageType        string   `json:"imageType"`
	Servings         int      `json:"servings"`
	ReadyInMinutes   int      `json:"readyInMinutes"`
	SourceUrl        string   `json:"sourceUrl"`
	Summary          string   `json:"summary"`
	Cuisines         []string `json:"cuisines"`
	DishTypes        []string `json:"dishTypes"`
	Diets            []string `json:"diets"`
}

// ============================================================================
// Fetch Methods
// ============================================================================

// FetchFoodNutrition fetches food nutrition data from USDA FoodData Central
func (f *FoodFetcher) FetchFoodNutrition(d *db.DB, query string, limit int) (int, error) {
	if f.USDAAPIKey == "" {
		return 0, fmt.Errorf("USDA_API_KEY not set; get a free key at https://fdc.nal.usda.gov/api-key-signup.html")
	}

	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("api_key", f.USDAAPIKey)
	params.Set("query", query)
	params.Set("pageSize", fmt.Sprintf("%d", min(limit, 200)))
	params.Set("dataType", "Foundation,SR Legacy,Survey (FNDDS),Branded")

	reqURL := f.USDABaseURL + "/foods/search?" + params.Encode()
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("USDA API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("USDA API returned status %d", resp.StatusCode)
	}

	var result usdaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode USDA response: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO food_items
		(fdc_id, name, brand, data_type, serving_size, serving_unit, calories, protein_g, fat_g, carbs_g, fiber_g, sugar_g, sodium_mg, ingredients)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO food_items_fts
		(fdc_id, name, brand, ingredients)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, food := range result.Foods {
		nutrients := extractNutrients(food.FoodNutrients)

		brand := food.BrandOwner
		if brand == "" {
			brand = food.BrandName
		}

		_, err := stmt.Exec(
			food.FdcId,
			food.Description,
			brand,
			food.DataType,
			food.ServingSize,
			food.ServingSizeUnit,
			nutrients["calories"],
			nutrients["protein"],
			nutrients["fat"],
			nutrients["carbs"],
			nutrients["fiber"],
			nutrients["sugar"],
			nutrients["sodium"],
			food.Ingredients,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(food.FdcId, food.Description, brand, food.Ingredients)
		count++
	}

	return count, nil
}

// extractNutrients extracts common nutrients from USDA nutrient list
func extractNutrients(nutrients []usdaNutrient) map[string]float64 {
	result := make(map[string]float64)

	// Map nutrient IDs to our standardized names
	// See: https://fdc.nal.usda.gov/api-spec/fdc_api.html
	nutrientMap := map[int]string{
		1008: "calories",  // Energy (kcal)
		1003: "protein",   // Protein
		1004: "fat",       // Total lipid (fat)
		1005: "carbs",     // Carbohydrate
		1079: "fiber",     // Fiber, total dietary
		2000: "sugar",     // Sugars, total including NLEA
		1093: "sodium",    // Sodium, Na
	}

	for _, n := range nutrients {
		if name, ok := nutrientMap[n.NutrientId]; ok {
			result[name] = n.Value
		}
	}

	return result
}

// FetchOpenFoodProducts fetches products from Open Food Facts (no API key required)
func (f *FoodFetcher) FetchOpenFoodProducts(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	// Open Food Facts uses a search endpoint
	params := url.Values{}
	params.Set("search_terms", query)
	params.Set("search_simple", "1")
	params.Set("action", "process")
	params.Set("json", "1")
	params.Set("page_size", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("fields", "code,product_name,brands,categories,ingredients_text,nutrition_grades,nutriments,serving_size,quantity,countries,nutriscore_score,image_url")

	reqURL := "https://world.openfoodfacts.org/cgi/search.pl?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mimir-food-fetcher/1.0 (contact: fools@fools.ai)")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Open Food Facts API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Open Food Facts API returned status %d", resp.StatusCode)
	}

	var result openFoodSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Open Food Facts response: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO food_items
		(fdc_id, name, brand, data_type, serving_size, serving_unit, calories, protein_g, fat_g, carbs_g, fiber_g, sugar_g, sodium_mg, ingredients)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO food_items_fts
		(fdc_id, name, brand, ingredients)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, product := range result.Products {
		if product.ProductName == "" {
			continue
		}

		// Use barcode (code) as ID with prefix to distinguish from USDA
		fdcId := "OFF_" + product.Code

		// Serving size from Open Food Facts is a string like "100g"
		servingSize := 100.0 // default
		servingUnit := "g"
		if product.ServingSize != "" {
			// Try to parse serving size
			fmt.Sscanf(product.ServingSize, "%f", &servingSize)
		}

		_, err := stmt.Exec(
			fdcId,
			product.ProductName,
			product.Brands,
			"Open Food Facts",
			servingSize,
			servingUnit,
			product.Nutriments.EnergyKcal100g,
			product.Nutriments.Proteins100g,
			product.Nutriments.Fat100g,
			product.Nutriments.Carbohydrates100g,
			product.Nutriments.Fiber100g,
			product.Nutriments.Sugars100g,
			product.Nutriments.Sodium100g*1000, // convert g to mg
			product.IngredientsText,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(fdcId, product.ProductName, product.Brands, product.IngredientsText)
		count++
	}

	return count, nil
}

// FetchRecipes fetches recipes from TheMealDB (free tier, no API key required)
func (f *FoodFetcher) FetchRecipes(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 25
	}

	// TheMealDB search endpoint
	params := url.Values{}
	params.Set("s", query)

	reqURL := f.TheMealDBURL + "/search.php?" + params.Encode()
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("TheMealDB API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("TheMealDB API returned status %d", resp.StatusCode)
	}

	var result mealDBSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode TheMealDB response: %w", err)
	}

	if result.Meals == nil {
		return 0, nil // No results
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes
		(recipe_id, name, category, cuisine, instructions, ingredients, tags, image_url, video_url, source_url, source_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes_fts
		(recipe_id, name, category, cuisine, ingredients, tags)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, meal := range result.Meals {
		if count >= limit {
			break
		}

		recipeId := "MDB_" + meal.IdMeal
		ingredients := meal.GetIngredients()

		_, err := stmt.Exec(
			recipeId,
			meal.StrMeal,
			meal.StrCategory,
			meal.StrArea,
			meal.StrInstructions,
			ingredients,
			meal.StrTags,
			meal.StrMealThumb,
			meal.StrYoutube,
			meal.StrSource,
			"TheMealDB",
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(recipeId, meal.StrMeal, meal.StrCategory, meal.StrArea, ingredients, meal.StrTags)
		count++
	}

	return count, nil
}

// FetchSpoonacularRecipes fetches recipes from Spoonacular (requires API key)
func (f *FoodFetcher) FetchSpoonacularRecipes(d *db.DB, query string, limit int) (int, error) {
	if f.SpoonacularAPIKey == "" {
		return 0, fmt.Errorf("SPOONACULAR_API_KEY not set; get a key at https://spoonacular.com/food-api")
	}

	if limit == 0 {
		limit = 25
	}

	params := url.Values{}
	params.Set("apiKey", f.SpoonacularAPIKey)
	params.Set("query", query)
	params.Set("number", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("addRecipeInformation", "true")

	reqURL := f.SpoonacularURL + "/recipes/complexSearch?" + params.Encode()
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("Spoonacular API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Spoonacular API returned status %d", resp.StatusCode)
	}

	var result spoonacularSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Spoonacular response: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes
		(recipe_id, name, category, cuisine, instructions, ingredients, tags, image_url, video_url, source_url, source_type, servings, prep_time_min)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes_fts
		(recipe_id, name, category, cuisine, ingredients, tags)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, recipe := range result.Results {
		recipeId := fmt.Sprintf("SPOON_%d", recipe.Id)
		category := strings.Join(recipe.DishTypes, "; ")
		cuisine := strings.Join(recipe.Cuisines, "; ")
		tags := strings.Join(recipe.Diets, "; ")

		_, err := stmt.Exec(
			recipeId,
			recipe.Title,
			category,
			cuisine,
			recipe.Summary, // Note: full instructions require separate API call
			"",             // Ingredients require separate API call
			tags,
			recipe.Image,
			"",
			recipe.SourceUrl,
			"Spoonacular",
			recipe.Servings,
			recipe.ReadyInMinutes,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(recipeId, recipe.Title, category, cuisine, "", tags)
		count++
	}

	return count, nil
}

// FetchRecipesByIngredients fetches recipes that use specific ingredients (Spoonacular)
func (f *FoodFetcher) FetchRecipesByIngredients(d *db.DB, ingredients []string, limit int) (int, error) {
	if f.SpoonacularAPIKey == "" {
		return 0, fmt.Errorf("SPOONACULAR_API_KEY not set")
	}

	if limit == 0 {
		limit = 10
	}

	params := url.Values{}
	params.Set("apiKey", f.SpoonacularAPIKey)
	params.Set("ingredients", strings.Join(ingredients, ","))
	params.Set("number", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("ranking", "2") // Maximize used ingredients

	reqURL := f.SpoonacularURL + "/recipes/findByIngredients?" + params.Encode()
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("Spoonacular API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Spoonacular API returned status %d", resp.StatusCode)
	}

	type ingredientResult struct {
		Id               int    `json:"id"`
		Title            string `json:"title"`
		Image            string `json:"image"`
		UsedIngredients  []struct{ Name string `json:"name"` } `json:"usedIngredients"`
		MissedIngredients []struct{ Name string `json:"name"` } `json:"missedIngredients"`
	}

	var results []ingredientResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, fmt.Errorf("failed to decode Spoonacular response: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes
		(recipe_id, name, category, cuisine, instructions, ingredients, tags, image_url, video_url, source_url, source_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO recipes_fts
		(recipe_id, name, category, cuisine, ingredients, tags)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, recipe := range results {
		recipeId := fmt.Sprintf("SPOON_%d", recipe.Id)

		var usedNames, missedNames []string
		for _, ing := range recipe.UsedIngredients {
			usedNames = append(usedNames, ing.Name)
		}
		for _, ing := range recipe.MissedIngredients {
			missedNames = append(missedNames, ing.Name)
		}

		allIngredients := strings.Join(append(usedNames, missedNames...), "; ")
		tags := fmt.Sprintf("uses: %s", strings.Join(usedNames, ", "))

		_, err := stmt.Exec(
			recipeId,
			recipe.Title,
			"",
			"",
			"",
			allIngredients,
			tags,
			recipe.Image,
			"",
			"",
			"Spoonacular",
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(recipeId, recipe.Title, "", "", allIngredients, tags)
		count++
	}

	return count, nil
}

// FetchMealDBByCategory fetches recipes by category from TheMealDB
func (f *FoodFetcher) FetchMealDBByCategory(d *db.DB, category string, limit int) (int, error) {
	if limit == 0 {
		limit = 25
	}

	// First, get list of meals in category
	reqURL := f.TheMealDBURL + "/filter.php?c=" + url.QueryEscape(category)
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return 0, fmt.Errorf("TheMealDB API request failed: %w", err)
	}
	defer resp.Body.Close()

	type mealSummary struct {
		IdMeal       string `json:"idMeal"`
		StrMeal      string `json:"strMeal"`
		StrMealThumb string `json:"strMealThumb"`
	}
	type filterResponse struct {
		Meals []mealSummary `json:"meals"`
	}

	var filterResult filterResponse
	if err := json.NewDecoder(resp.Body).Decode(&filterResult); err != nil {
		return 0, fmt.Errorf("failed to decode TheMealDB response: %w", err)
	}

	if filterResult.Meals == nil {
		return 0, nil
	}

	// Now fetch full details for each meal
	count := 0
	for _, mealSummary := range filterResult.Meals {
		if count >= limit {
			break
		}

		detailResp, err := f.client.Get(f.TheMealDBURL + "/lookup.php?i=" + mealSummary.IdMeal)
		if err != nil {
			continue
		}

		var detailResult mealDBSearchResponse
		if err := json.NewDecoder(detailResp.Body).Decode(&detailResult); err != nil {
			detailResp.Body.Close()
			continue
		}
		detailResp.Body.Close()

		if len(detailResult.Meals) == 0 {
			continue
		}

		meal := detailResult.Meals[0]
		recipeId := "MDB_" + meal.IdMeal
		ingredients := meal.GetIngredients()

		stmt, _ := d.Prepare(`INSERT OR REPLACE INTO recipes
			(recipe_id, name, category, cuisine, instructions, ingredients, tags, image_url, video_url, source_url, source_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)

		_, err = stmt.Exec(
			recipeId,
			meal.StrMeal,
			meal.StrCategory,
			meal.StrArea,
			meal.StrInstructions,
			ingredients,
			meal.StrTags,
			meal.StrMealThumb,
			meal.StrYoutube,
			meal.StrSource,
			"TheMealDB",
		)
		stmt.Close()
		if err != nil {
			continue
		}

		ftsStmt, _ := d.Prepare(`INSERT OR REPLACE INTO recipes_fts
			(recipe_id, name, category, cuisine, ingredients, tags)
			VALUES (?, ?, ?, ?, ?, ?)`)
		ftsStmt.Exec(recipeId, meal.StrMeal, meal.StrCategory, meal.StrArea, ingredients, meal.StrTags)
		ftsStmt.Close()

		count++
	}

	return count, nil
}

// GetMealDBCategories returns available meal categories from TheMealDB
func (f *FoodFetcher) GetMealDBCategories() ([]string, error) {
	reqURL := f.TheMealDBURL + "/categories.php"
	resp, err := f.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type category struct {
		IdCategory             string `json:"idCategory"`
		StrCategory            string `json:"strCategory"`
		StrCategoryThumb       string `json:"strCategoryThumb"`
		StrCategoryDescription string `json:"strCategoryDescription"`
	}
	type categoriesResponse struct {
		Categories []category `json:"categories"`
	}

	var result categoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	categories := make([]string, len(result.Categories))
	for i, cat := range result.Categories {
		categories[i] = cat.StrCategory
	}

	return categories, nil
}

// FoodFetchResult holds results from food data fetch
type FoodFetchResult struct {
	OpenFoodFacts int `json:"open_food_facts"`
	TheMealDB     int `json:"the_meal_db"`
	USDA          int `json:"usda"`
	Spoonacular   int `json:"spoonacular"`
}

// FetchAll fetches from all available food sources
// Key-free: Open Food Facts, TheMealDB
// Key-required: USDA, Spoonacular (skipped if no key)
func (f *FoodFetcher) FetchAll(d *db.DB, query string, limit int) (*FoodFetchResult, error) {
	result := &FoodFetchResult{}

	// Open Food Facts - NO KEY REQUIRED (priority)
	if count, err := f.FetchOpenFoodProducts(d, query, limit); err == nil {
		result.OpenFoodFacts = count
	}

	// TheMealDB - NO KEY REQUIRED
	if count, err := f.FetchRecipes(d, query, limit); err == nil {
		result.TheMealDB = count
	}

	// USDA - requires key (skip if not set)
	if f.USDAAPIKey != "" {
		if count, err := f.FetchFoodNutrition(d, query, limit); err == nil {
			result.USDA = count
		}
	}

	// Spoonacular - requires key (skip if not set)
	if f.SpoonacularAPIKey != "" {
		if count, err := f.FetchSpoonacularRecipes(d, query, limit); err == nil {
			result.Spoonacular = count
		}
	}

	return result, nil
}

// AvailableSources returns which food sources are available
func (f *FoodFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"open_food_facts": true,                       // Always available
		"the_meal_db":     true,                       // Always available
		"usda":            f.USDAAPIKey != "",         // Requires USDA_API_KEY
		"spoonacular":     f.SpoonacularAPIKey != "",  // Requires SPOONACULAR_API_KEY
	}
}
