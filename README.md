# PLS

PLS is a property listing system where users can list their properties of any type for sale or for rent and other users can see the available properties, mark them as favroties and recommend properties to other registered users. 

## Features

- **Property Management**: Fetch, Add, Edit, and Delete property details seamlessly.
- **Adavanced Filtering**: Users can put filters on 10+ attributes and filter out the properties of their choice.
- **Favorties**: Users can mark specific properties as favorites and access them under one separate section.
- **Recommendation System**: Users can recommend properties to other registered users and also see the properties that are recommended to them by others.
- **Caching**: Caching is added to decrease the number of database calls and better API performance.


## Tech Stack

- **Backend**: `GoLang` for developing APIs and managing http requests, `MongoDB` as the database, `Redis` is used for caching and `JWT` is used for authentication.

## Installation and Setup

Follow these steps to set up the project locally:

### Prerequisites

- Golang
- MongoDB
- Redis

### Backend Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/dcode-github/property_listing_system.git
   cd property_listing_system/backend
   ```
2. Create a `.env` file and add the required environment variable values.

3. Configure the database:

   - Update the database credentials in the `config` directory.

4. Install dependencies and run the server:

   ```bash
   go mod tidy
   go run main.go
   ```

## API Endpoints

### Auth API

- **POST `/login`**
  - Check the login credentials of the user.
  - Request Body: `userID`,`password`
- **POST `/register`**
  - Add new user to database.
  - Request Body: `userID`,`email`,`password`

### Properties APIs

- **GET `/api/properties`**
  - Fetch all properties.
  - Query Params: `filters`(optional)
- **POST `/api/properties`**
  - Add a property in the database.
  - Request Body: refer `backend/models/property.go` for the schema
- **PUT `/api/properties/{id}`**
  - Update the property `id` if the property is created by the user
  - Request Body: refer `backend/models/property.go` for the schema
- **DELETE `/api/properties/{id}`**
  - Delete a property if the property is created by the user.
  - Query Params: `id`


### Favorites APIs

- **GET `/api/favorites`**
  - Fetch all favorite properties of the `user`.
- **POST `/api/favorites`**
  - Add a property as favorite under the `user`.
  - Request Body: `userID`,`propertyID`
- **DELETE `/api/favorites/{id}`**
  - Remove the property from favorite.
  - Query Params: `id`


### Recommendations APIs

- **GET `/api/recommendations`**
  - Fetch all the recommendations recieved for the `user`.
- **POST `/api/recommend`**
  - Recommend a property to a registered user.
  - Request Body: `fromUserID`,`toUserID`,`toEmailID`,`propertyID`




## Contributing

We welcome contributions to EquiTrack! Here's how you can get involved:

1. Fork the repository.
2. Create a new branch: `git checkout -b feature-name`.
3. Commit your changes: `git commit -m 'Add some feature'`.
4. Push to the branch: `git push origin feature-name`.
5. Open a pull request.


## Contact

For questions or support, please contact [danish.eqbal125@gmail.com](mailto\:danish.eqbal125@gmail.com).

