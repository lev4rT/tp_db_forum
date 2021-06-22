CREATE EXTENSION IF NOT EXISTS citext;

DROP TABLE IF EXISTS users CASCADE;
CREATE UNLOGGED TABLE users (
                       nickname CITEXT UNIQUE PRIMARY KEY,  -- Имя пользователя (уникальное поле). Данное поле допускает только латиницу, цифры и знак подчеркивания. Сравнение имени регистронезависимо.
                       fullname TEXT NOT NULL,  -- Полное имя пользователя.
                       about TEXT,  -- Описание пользователя.
                       email CITEXT NOT NULL UNIQUE -- Почтовый адрес пользователя (уникальное поле).
);
DROP INDEX IF EXISTS usersNickname;
CREATE INDEX IF NOT EXISTS usersNickname ON users USING HASH (nickname);

DROP INDEX IF EXISTS usersEmail;
CREATE INDEX IF NOT EXISTS usersEmail ON users USING HASH (email);

DROP TABLE IF EXISTS forums CASCADE;
CREATE UNLOGGED TABLE forums (
                        title TEXT NOT NULL,  -- Название форума.
                        "user" CITEXT NOT NULL REFERENCES users(nickname) ,  -- Nickname пользователя, который отвечает за форум.
                        slug CITEXT NOT NULL UNIQUE PRIMARY KEY,  -- Человекопонятный URL. Уникальное поле.
                        posts BIGINT DEFAULT 0,  -- Общее кол-во сообщений в данном форуме.
                        threads BIGINT DEFAULT 0  -- Общее кол-во ветвей обсуждения в данном форуме.
);
DROP INDEX IF EXISTS forumsSlug;
CREATE INDEX forumsSlug ON forums USING HASH (slug);

DROP INDEX IF EXISTS forumsUser;
CREATE INDEX forumsUser ON forums ("user");

DROP TABLE IF EXISTS threads CASCADE;
CREATE UNLOGGED TABLE threads (
                         id SERIAL NOT NULL PRIMARY KEY,  -- Идентификатор ветки обсуждения.
                         title TEXT NOT NULL,  -- Заголовок ветки обсуждения.
                         author CITEXT NOT NULL REFERENCES users(nickname),  -- Пользователь, создавший данную тему.
                         forum CITEXT NOT NULL REFERENCES forums(slug),  -- Форум, в котором расположена данная ветка обсуждения.
                         message TEXT NOT NULL,  -- Описание ветки обсуждения.
                         votes INTEGER DEFAULT 0,  -- Кол-во голосов непосредственно за данное сообщение форума.
                         slug CITEXT,  -- Человекопонятный URL. В данной структуре slug опционален и не может быть числом.
                         created TIMESTAMP WITH TIME ZONE DEFAULT NOW()  -- Дата создания ветки на форуме.
);
DROP INDEX IF EXISTS threadsForum;
CREATE INDEX IF NOT EXISTS threadsForum ON threads USING HASH (forum);

DROP INDEX IF EXISTS threadsSlug;
CREATE INDEX IF NOT EXISTS threadsSlug ON threads USING HASH (slug);

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION appendThreadsCounterForum() RETURNS TRIGGER AS
$$
DECLARE
    nicknameUser CITEXT;
    fullnameUser TEXT;
    aboutUser TEXT;
    emailUser CITEXT;
BEGIN
    UPDATE forums SET threads = threads + 1 WHERE slug = NEW.forum;
    SELECT nickname, fullname, about, email INTO nicknameUser, fullnameUser, aboutUser, emailUser FROM users WHERE nickname = NEW.author;
    INSERT INTO usersonforums(nickname, fullname, about, email, slug) VALUES (nicknameUser, fullnameUser, aboutUser, emailUser, NEW.forum) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER appendThreadsCounterForumTrigger AFTER INSERT ON "threads"
    FOR EACH ROW
    EXECUTE PROCEDURE appendThreadsCounterForum();

------------------------------------------------------------------------

DROP TABLE IF EXISTS posts CASCADE;
CREATE UNLOGGED TABLE posts (
                       id BIGSERIAL NOT NULL PRIMARY KEY,  -- Идентификатор данного сообщения.
                       parent BIGINT DEFAULT 0,  -- Идентификатор родительского сообщения (0 - корневое сообщение обсуждения).
                       author CITEXT NOT NULL REFERENCES users(nickname),  -- Автор, написавший данное сообщение.
                       message TEXT NOT NULL,  -- Собственно сообщение форума.
                       isEdited BOOLEAN DEFAULT false,  -- Истина, если данное сообщение было изменено.
                       forum CITEXT, -- NOT NULL REFERENCES forums(slug),  -- Идентификатор форума (slug) данного сообещния.
                       thread INTEGER REFERENCES threads(id),  -- Идентификатор ветви (id) обсуждения данного сообещния.
                       created TIMESTAMP WITH TIME ZONE DEFAULT NOW(),  -- Дата создания сообщения на форуме.
                       path BIGINT[] DEFAULT '{}', -- Materialized Path. Используется для вложенных постов

                       CONSTRAINT unique_post UNIQUE (author, message, forum, thread)
);
DROP INDEX IF EXISTS postsThreadID;
-- CREATE INDEX IF NOT EXISTS postsThreadID ON posts (thread, id);
--
DROP INDEX IF EXISTS postsPath1DescID;
-- CREATE INDEX IF NOT EXISTS postsPath1DescID ON posts ((path[1]) DESC, id);
--
DROP INDEX IF EXISTS postsThreadIDPath1Parent;
-- CREATE INDEX IF NOT EXISTS postsThreadIDPath1Parent ON posts (thread, id, (path[1]), parent);
--
DROP INDEX IF EXISTS postsThreadPathID;
-- CREATE INDEX IF NOT EXISTS postsThreadPathID ON posts (thread, path, id);


------------------------------------------------------------------------

-- CREATE OR REPLACE FUNCTION appendPostsCounterForum() RETURNS TRIGGER AS
-- $$
-- DECLARE
--     nicknameUser CITEXT;
--     fullnameUser TEXT;
--     aboutUser TEXT;
--     emailUser CITEXT;
-- BEGIN
--     UPDATE forums SET posts = posts + 1 WHERE slug = NEW.forum;
--     SELECT nickname, fullname, about, email INTO nicknameUser, fullnameUser, aboutUser, emailUser FROM users WHERE nickname = NEW.author;
--     INSERT INTO usersonforums(nickname, fullname, about, email, slug) VALUES (nicknameUser, fullnameUser, aboutUser, emailUser, NEW.forum) ON CONFLICT DO NOTHING;
--     RETURN NEW;
-- END;
-- $$
-- LANGUAGE 'plpgsql';
--
-- CREATE TRIGGER appendPostsCounterForumTrigger AFTER INSERT ON "posts"
--     FOR EACH ROW
--     EXECUTE PROCEDURE appendPostsCounterForum();

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION setPathForPost() RETURNS TRIGGER AS
$$
DECLARE
    nicknameUser CITEXT;
    fullnameUser TEXT;
    aboutUser TEXT;
    emailUser CITEXT;
BEGIN
    IF NEW.parent = 0 THEN
        NEW.path = ARRAY [NEW.id];
    ELSE
        SELECT path INTO NEW.PATH FROM posts WHERE id = NEW.parent;
        NEW.path = array_append(NEW.path, NEW.id);
    END IF;
    UPDATE forums SET posts = posts + 1 WHERE slug = NEW.forum;
    SELECT nickname, fullname, about, email INTO nicknameUser, fullnameUser, aboutUser, emailUser FROM users WHERE nickname = NEW.author;
    INSERT INTO usersonforums(nickname, fullname, about, email, slug) VALUES (nicknameUser, fullnameUser, aboutUser, emailUser, NEW.forum) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER setPathForPostTrigger BEFORE INSERT ON "posts"
    FOR EACH ROW
    EXECUTE PROCEDURE setPathForPost();

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION setPostIsEdited() RETURNS TRIGGER AS
$$
BEGIN
    IF NEW.message = '' OR NEW.message = OLD.message THEN
        RETURN NEW;
    ELSE
        NEW.isedited=true;
        RETURN NEW;
    END IF;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER setPostIsEditedTrigger BEFORE UPDATE ON "posts"
    FOR EACH ROW
    EXECUTE PROCEDURE setPostIsEdited();

DROP TABLE IF EXISTS votes CASCADE;
CREATE UNLOGGED TABLE votes(
                      nickname CITEXT NOT NULL REFERENCES users(nickname),  -- Идентификатор пользователя.
                      voice SMALLINT,  -- Отданный голос.
                      threadID INT REFERENCES threads(id),  -- ID  треда

                      CONSTRAINT uniqueVote UNIQUE (nickname, threadID)
);
-- CREATE UNIQUE INDEX OF NOT EXISTS votesNicknameThreadID ON votes(nickname, threadID)

CREATE OR REPLACE FUNCTION addVoteForThread() RETURNS TRIGGER AS
$$
BEGIN
    UPDATE threads SET votes = votes + NEW."voice" WHERE id = NEW."threadid";
RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER addVoteForThreadTrigger AFTER INSERT ON "votes"
    FOR EACH ROW
    EXECUTE PROCEDURE addVoteForThread();

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION changeVoteForThread() RETURNS TRIGGER AS
$$
BEGIN
    IF OLD.voice = NEW.voice THEN
        RETURN OLD;
    ELSE
        UPDATE threads SET votes = votes + 2 * NEW."voice" WHERE id = NEW."threadid";
        RETURN NEW;
    END IF;
END;
$$
LANGUAGE 'plpgsql';

DROP TRIGGER IF EXISTS changeVoteForThreadTrigger ON "votes" CASCADE;
CREATE TRIGGER changeVoteForThreadTrigger AFTER UPDATE ON "votes"
    FOR EACH ROW
    EXECUTE PROCEDURE changeVoteForThread();


------------------------------------------------------------------------

DROP TABLE IF EXISTS usersOnForums CASCADE;
CREATE UNLOGGED TABLE usersOnForums (
    nickname CITEXT NOT NULL REFERENCES users(nickname),  -- Имя пользователя (уникальное поле). Данное поле допускает только латиницу, цифры и знак подчеркивания. Сравнение имени регистронезависимо.
    fullname TEXT NOT NULL,  -- Полное имя пользователя.
    about TEXT,  -- Описание пользователя.
    email CITEXT NOT NULL,  -- Почтовый адрес пользователя (уникальное поле).
    slug CITEXT NOT NULL REFERENCES forums(slug)  -- Человекопонятный URL. Уникальное поле.
);

DROP INDEX IF EXISTS usersOnForumsNicknameSlug;
CREATE UNIQUE INDEX IF NOT EXISTS usersOnForumsNicknameSlug ON usersOnForums(slug, nickname);