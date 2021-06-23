CREATE EXTENSION IF NOT EXISTS citext;

DROP TABLE IF EXISTS users CASCADE;
CREATE UNLOGGED TABLE users (
                       nickname CITEXT UNIQUE PRIMARY KEY,  -- Имя пользователя (уникальное поле). Данное поле допускает только латиницу, цифры и знак подчеркивания. Сравнение имени регистронезависимо.
                       fullname TEXT NOT NULL,  -- Полное имя пользователя.
                       about TEXT,  -- Описание пользователя.
                       email CITEXT NOT NULL UNIQUE -- Почтовый адрес пользователя (уникальное поле).
);
DROP INDEX IF EXISTS usersEmail;
CREATE INDEX usersEmail ON users(email);

DROP TABLE IF EXISTS forums CASCADE;
CREATE UNLOGGED TABLE forums (
                        title TEXT NOT NULL,  -- Название форума.
                        "user" CITEXT NOT NULL REFERENCES users(nickname) ,  -- Nickname пользователя, который отвечает за форум.
                        slug CITEXT NOT NULL UNIQUE PRIMARY KEY,  -- Человекопонятный URL. Уникальное поле.
                        posts INT DEFAULT 0,  -- Общее кол-во сообщений в данном форуме.
                        threads INT DEFAULT 0  -- Общее кол-во ветвей обсуждения в данном форуме.
);

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
CREATE INDEX threadsForum ON threads(forum);

DROP INDEX IF EXISTS threadsSlug;
CREATE UNIQUE INDEX threadsSlug ON threads(slug) WHERE slug IS NOT NULL;


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
INSERT INTO usersonforums(nickname, slug) VALUES (nicknameUser, NEW.forum) ON CONFLICT DO NOTHING;
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
                       id SERIAL PRIMARY KEY,  -- Идентификатор данного сообщения.
                       parent INTEGER REFERENCES posts(id) DEFAULT NULL,  -- Идентификатор родительского сообщения (0 - корневое сообщение обсуждения).
                       author CITEXT NOT NULL REFERENCES users(nickname),  -- Автор, написавший данное сообщение.
                       message TEXT NOT NULL,  -- Собственно сообщение форума.
                       isEdited BOOLEAN DEFAULT FALSE,  -- Истина, если данное сообщение было изменено.
                       forum CITEXT REFERENCES forums(slug), -- NOT NULL REFERENCES forums(slug),  -- Идентификатор форума (slug) данного сообещния.
                       thread INTEGER REFERENCES threads(id),  -- Идентификатор ветви (id) обсуждения данного сообещния.
                       created TIMESTAMP WITH TIME ZONE,  -- Дата создания сообщения на форуме.
                       path INTEGER[], -- Materialized Path. Используется для вложенных постов

                       CONSTRAINT unique_post UNIQUE (author, message, forum, thread)
);

DROP INDEX IF EXISTS postsThread;
-- create index postsThread on posts (thread);

DROP INDEX IF EXISTS postsThreadID;
-- CREATE INDEX IF NOT EXISTS postsThreadID ON posts (thread, id);

DROP INDEX IF EXISTS postsPathID;
-- CREATE INDEX IF NOT EXISTS postsPathID ON posts (path, id);

DROP INDEX IF EXISTS postsThreadPathID;
-- CREATE INDEX IF NOT EXISTS postsThreadPathID ON posts (thread, path, id);
-- --
-- DROP INDEX IF EXISTS postsPath1DescID;
-- CREATE INDEX IF NOT EXISTS postsPath1DescID ON posts ((path[1]) DESC, id);
-- --
-- DROP INDEX IF EXISTS postsThreadIDPath1Parent;
-- CREATE INDEX IF NOT EXISTS postsThreadIDPath1Parent ON posts (thread, id, (path[1]), parent);
-- --
-- DROP INDEX IF EXISTS postsThreadPathID;
-- CREATE INDEX IF NOT EXISTS postsThreadPathID ON posts (thread, path, id);


------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION appendPostsCounterForum() RETURNS TRIGGER AS
$$
DECLARE
    nicknameUser CITEXT;
BEGIN
    UPDATE forums SET posts = posts + 1 WHERE slug = NEW.forum;
    SELECT nickname INTO nicknameUser FROM users WHERE nickname = NEW.author;
    INSERT INTO usersonforums(nickname, slug) VALUES (nicknameUser, NEW.forum) ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER appendPostsCounterForumTrigger AFTER INSERT ON "posts"
    FOR EACH ROW
    EXECUTE PROCEDURE appendPostsCounterForum();

------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION setPathForPost() RETURNS TRIGGER AS
$$
DECLARE
    parents INTEGER [];
    parentsThread INTEGER;
BEGIN
    if (new.parent is null) then
        new.path := new.path || new.id;
    else
        select path, thread from posts where id = new.parent into parents, parentsThread;
        if (coalesce(array_length(parents, 1), 0) = 0) then
            raise exception 'parents post does not exists' USING ERRCODE = '77777';
        end if;
        if parentsThread != new.thread then
            raise exception 'threads are different' USING ERRCODE = '77778';
        end if;

        new.path := new.path || parents || new.id;
    end if;
    return new;
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
                      threadID INT REFERENCES threads(id)  -- ID  треда
);
DROP INDEX IF EXISTS votesNicknameThreadID;
CREATE UNIQUE INDEX votesNicknameThreadID ON votes(nickname, threadID);

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
    id SERIAL NOT NULL PRIMARY KEY,
    nickname CITEXT NOT NULL REFERENCES users(nickname),  -- Имя пользователя (уникальное поле). Данное поле допускает только латиницу, цифры и знак подчеркивания. Сравнение имени регистронезависимо.
    slug CITEXT NOT NULL REFERENCES forums(slug),  -- Человекопонятный URL. Уникальное поле.

    CONSTRAINT uniqueUserOnForum UNIQUE (nickname, slug)
);

DROP INDEX IF EXISTS usersOnForumsNicknameSlug;
CREATE UNIQUE INDEX IF NOT EXISTS usersOnForumsNicknameSlug ON usersOnForums(nickname, slug);

ANALYZE users;
ANALYZE forums;
ANALYZE threads;
ANALYZE posts;
ANALYZE votes;
ANALYZE usersOnForums;
