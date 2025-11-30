import requests
import zipfile
import io
import sys

schema = """
"""
A user in the system
"""
type User {
  id: ID!
  username: String!
  email: String!
  role: UserRole!
  profile: UserProfile
  posts: [Post!]!
  comments: [Comment!]!
  createdAt: Time!
  updatedAt: Time!
}

"""
User profile information
"""
type UserProfile {
  bio: String
  avatar: String
  website: String
  location: String
  socialLinks: [SocialLink!]!
}

"""
Social media link
"""
type SocialLink {
  platform: SocialPlatform!
  url: String!
}

"""
A blog post
"""
type Post implements Content {
  id: ID!
  title: String!
  body: String!
  author: User!
  tags: [String!]!
  status: PostStatus!
  comments: [Comment!]!
  likes: Int!
  views: Int!
  createdAt: Time!
  updatedAt: Time!
  publishedAt: Time
}

"""
A comment on a post
"""
type Comment implements Content {
  id: ID!
  body: String!
  author: User!
  post: Post!
  parent: Comment
  replies: [Comment!]!
  likes: Int!
  createdAt: Time!
  updatedAt: Time!
}

"""
Content interface for posts and comments
"""
interface Content {
  id: ID!
  body: String!
  author: User!
  likes: Int!
  createdAt: Time!
  updatedAt: Time!
}

"""
Search result union
"""
union SearchResult = User | Post | Comment

"""
User role enum
"""
enum UserRole {
  ADMIN
  MODERATOR
  USER
  GUEST
}

"""
Post status enum
"""
enum PostStatus {
  DRAFT
  PUBLISHED
  ARCHIVED
}

"""
Social platform enum
"""
enum SocialPlatform {
  TWITTER
  GITHUB
  LINKEDIN
  FACEBOOK
  INSTAGRAM
}

"""
Input for creating a user
"""
input CreateUserInput {
  username: String!
  email: String!
  password: String!
  role: UserRole
}

"""
Input for updating a user
"""
input UpdateUserInput {
  username: String
  email: String
  role: UserRole
  profile: UpdateProfileInput
}

"""
Input for updating profile
"""
input UpdateProfileInput {
  bio: String
  avatar: String
  website: String
  location: String
}

"""
Input for creating a post
"""
input CreatePostInput {
  title: String!
  body: String!
  tags: [String!]
  status: PostStatus
}

"""
Input for creating a comment
"""
input CreateCommentInput {
  postId: ID!
  body: String!
  parentId: ID
}

"""
Pagination input
"""
input PaginationInput {
  page: Int
  limit: Int
}

"""
Filter input for posts
"""
input PostFilterInput {
  authorId: ID
  status: PostStatus
  tags: [String!]
}

"""
Paginated response
"""
type PaginatedPosts {
  posts: [Post!]!
  total: Int!
  page: Int!
  hasMore: Boolean!
}

"""
Custom scalar for time
"""
scalar Time

type Query {
  """
  Get current user
  """
  me: User
  
  """
  Get user by ID
  """
  user(id: ID!): User
  
  """
  Get all users with pagination
  """
  users(pagination: PaginationInput): [User!]!
  
  """
  Get post by ID
  """
  post(id: ID!): Post
  
  """
  Get posts with filters and pagination
  """
  posts(filter: PostFilterInput, pagination: PaginationInput): PaginatedPosts!
  
  """
  Search across users, posts, and comments
  """
  search(query: String!): [SearchResult!]!
  
  """
  Get comment by ID
  """
  comment(id: ID!): Comment
}

type Mutation {
  """
  Create a new user
  """
  createUser(input: CreateUserInput!): User!
  
  """
  Update user
  """
  updateUser(id: ID!, input: UpdateUserInput!): User!
  
  """
  Delete user
  """
  deleteUser(id: ID!): Boolean!
  
  """
  Create a new post
  """
  createPost(input: CreatePostInput!): Post!
  
  """
  Update post
  """
  updatePost(id: ID!, input: CreatePostInput!): Post!
  
  """
  Delete post
  """
  deletePost(id: ID!): Boolean!
  
  """
  Create a comment
  """
  createComment(input: CreateCommentInput!): Comment!
  
  """
  Like a post or comment
  """
  like(contentId: ID!): Content!
}

type Subscription {
  """
  Subscribe to new posts
  """
  postCreated: Post!
  
  """
  Subscribe to new comments on a post
  """
  commentAdded(postId: ID!): Comment!
}
"""

url = "http://localhost:8088/api/generate/zip"
payload = {"schema": schema, "module": "github.com/example/complex"}

try:
    response = requests.post(url, json=payload)
    response.raise_for_status()
    
    with zipfile.ZipFile(io.BytesIO(response.content)) as z:
        print("Files in zip:")
        for name in z.namelist():
            print(f" - {name}")
            
        if "generated/models_gen.go" in z.namelist():
            print("\nContent of generated/models_gen.go:")
            print(z.read("generated/models_gen.go").decode("utf-8")[:1000]) # Print first 1000 chars
        else:
            print("\ngenerated/models_gen.go NOT FOUND")
            sys.exit(1)
            
except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)
